package kcp

import (
  "io"
  "bytes"
  "errors"
)

const (
  KCP_RTO_NDL = 30 
  KCP_RTO_MIN = 100
  KCP_RTO_DEF = 200
  KCP_RTO_MAX = 60000
  KCP_WND_SND = 32
  KCP_WND_RCV = 32
  KCP_MTU_DEF = 1400
  KCP_ACK_FAST = 3
  KCP_INTERVAL = 100
  KCP_OVERHEAD = 4 * 8
  KCP_DEADLINK = 10
  KCP_THRESH_INIT = 2
  KCP_THRESH_MIN = 2
)

const (
  KCP_CMD_PUSH = 81
  KCP_CMD_ACK = 82
  KCP_CMD_WASK = 83
  KCP_CMD_WINS = 84
)

const (
  KCP_ASK_SEND = 1 
  KCP_ASK_TELL = 2 
  KCP_PROBE_INIT = 7000 
  KCP_PROBE_LIMIT = 120000 
)

type KCP struct {
  conv, mtu, mss, state uint32
  snd_una, snd_nxt, rcv_nxt uint32
  ts_recent, ts_lastack, ssthresh uint32
  rx_rttval, rx_srtt, rx_rto, rx_minrto uint32
  snd_wnd, rcv_wnd, rmt_wnd, cwnd, probe uint32
  current, interval, ts_flush, xmit uint32
  nodelay, updated uint32
  ts_probe, probe_wait uint32
  dead_link, incr uint32
  snd_queue, snd_buf *Queue
  rcv_queue, rcv_buf *Queue
  acklist []uint32
  buffer []byte
  faskresend uint32
  nocwnd uint32
  writer io.Writer
}

func min(a, b uint32) uint32 {
  if a < b {
    return a
  }
  return b
}

func max(a, b uint32) uint32 {
  if a > b {
    return a
  }
  return b
}

func bound(lower, middle, upper uint32) uint32 {
  return min(max(lower, middle), upper)
}

func timediff(later, earlier uint32) int {
  return int(later) - int(earlier)
}

func NewKCP(conv uint32, writer io.Writer) *KCP {
  kcp := new(KCP)
  kcp.init(conv, writer)
  return kcp
}

func (kcp *KCP) init(conv uint32, writer io.Writer) {
  kcp.conv = conv
  kcp.writer = writer
  kcp.snd_wnd, kcp.rcv_wnd, kcp.rmt_wnd, kcp.cwnd = KCP_WND_SND, KCP_WND_RCV, KCP_WND_RCV, 1
  kcp.mtu = KCP_MTU_DEF
  kcp.mss = KCP_MTU_DEF - KCP_OVERHEAD
  
  kcp.buffer = make([]byte, kcp.mtu + KCP_OVERHEAD)
  kcp.snd_queue, kcp.snd_buf = NewQueue(), NewQueue()
  kcp.rcv_queue, kcp.rcv_buf = NewQueue(), NewQueue()
  
  kcp.rx_rto, kcp.rx_minrto = KCP_RTO_DEF, KCP_RTO_MIN
  kcp.interval, kcp.ts_flush = KCP_INTERVAL, KCP_INTERVAL
  kcp.ssthresh = KCP_THRESH_INIT
  kcp.dead_link = KCP_DEADLINK
  kcp.state = 1
}

func (kcp *KCP) output(data []byte) error {
  if len(data) == 0 {
    return nil
  }
  cnt, err := kcp.writer.Write(data)
  if cnt != len(data) {
    return errors.New("less data sent")
  }
  return err
}

func (kcp *KCP) receive(peek bool) ([]byte, error) {
  _, err := kcp.peeksize()
  recover := false
  
  if err != nil {
    return nil, err
  }
  
  if kcp.rcv_queue.Len() >= kcp.rcv_wnd {
    recover = true
  }
  
  // merge all data
  var buffer bytes.Buffer
  for entry := kcp.rcv_queue.next; entry != kcp.rcv_queue; {
    seg := entry.val.(*Segment)
    entry = entry.next
    buffer.Write(seg.data)
    if peek {
      continue
    }
    
    kcp.rcv_queue.Delete(entry)
    if seg.frg == 0 {
      break
    }
  } 
  
  // Move data from snd_buf into snd_queue
  for kcp.rcv_buf.Len() > 0 {
    entry := kcp.rcv_buf.next
    seg := entry.val.(*Segment)
    if seg.sn == kcp.rcv_nxt && kcp.rcv_queue.Len() < kcp.rcv_wnd {
      kcp.rcv_buf.Delete(entry)
      kcp.rcv_queue.PushNode(entry)
      kcp.rcv_nxt++
    } else {
      break
    }
  }
  
  // tell remote side starting send data again
  if kcp.rcv_queue.Len() < kcp.rcv_wnd && recover {
    kcp.probe |= KCP_ASK_TELL
  }
  return buffer.Bytes(), nil
}

func (kcp *KCP) peeksize() (uint32, error) {
  if kcp.rcv_queue.Len() == 0 {
    return 0, errors.New("empty queue")
  }

  seg := kcp.rcv_queue.next.val.(*Segment)
  if seg.frg == 0 {
    return seg.len, nil
  } else if seg.frg + 1 > kcp.rcv_queue.Len() {
    return 0, errors.New("rcv data not enough")
  }
  
  var rslt uint32
  for entry := kcp.rcv_queue.next; entry != kcp.rcv_queue; entry = entry.next {
    rslt += entry.val.(*Segment).len
    if entry.val.(*Segment).frg == 0 {
      break
    }
  } 
  return rslt, nil
}

func (kcp *KCP) send(data []byte) error {
  if data == nil || len(data) == 0 {
    return errors.New("empty data")
  }
  dlen := uint32(len(data))
  
  count := (dlen + kcp.mss - 1) / kcp.mss
  if count > 255 {
    return errors.New("data size too large")
  } else if count == 0 {
    count = 1
  }
  
  for i := uint32(0); i < count; i++ {
    seg := NewSegment(kcp)
    if i == count - 1 {
      seg.data = data
    } else {
      seg.data = data[:kcp.mss]
      data = data[kcp.mss:]
    }
    seg.frg = count - i - 1
    seg.len = uint32(len(seg.data))
    kcp.snd_queue.Push(seg)
  }
  return nil
}


// calculate rtt and rto
func (kcp *KCP) update_ack(rtt uint32) {
  var rto uint32 = 0
  if kcp.rx_srtt == 0 {
    kcp.rx_srtt = rtt
    kcp.rx_rttval = rtt / 2
  } else {
    var delta uint32
    if rtt < kcp.rx_srtt {
      delta = kcp.rx_srtt - rtt
    } else {
      delta = rtt - kcp.rx_srtt
    }
    
    kcp.rx_rttval = (3 * kcp.rx_rttval + delta) / 4
    kcp.rx_srtt = (7 * kcp.rx_srtt + rtt) / 8
    if kcp.rx_srtt < 1 {
      kcp.rx_srtt = 1
    }
  }
  
  rto = kcp.rx_srtt + max(1, 4 * kcp.rx_rttval)
  kcp.rx_rto = bound(kcp.rx_minrto, rto, KCP_RTO_MAX)
}

// sync snd_una with snd_buf, it may not sync for ack will delete entry from snd_buf
func (kcp *KCP) shrink_buf() {
  if kcp.snd_buf.Len() == 0 {
    kcp.snd_una = kcp.snd_nxt
  } else {
    kcp.snd_una = kcp.snd_buf.next.val.(*Segment).sn
  }
}

// rcv ack from remote side
func (kcp *KCP) parse_ack(sn uint32) {
  // ack for old pkg or future pkg is invalid
  if sn < kcp.snd_una || sn <= kcp.snd_nxt {
    return
  }
  
  // got ack for queueed segment
  for entry := kcp.snd_buf.next; entry != kcp.snd_buf; entry = entry.next {
    seg := entry.val.(*Segment)
    if seg.sn < sn {
      seg.fastack++
    } else {
      kcp.snd_buf.Delete(entry)
      break 
    }
  }
}

// rcv una from remote side
func (kcp *KCP) parse_una(una uint32) {
  // una for old pkg or future pkg is invalid
  if una < kcp.snd_una || una >= kcp.snd_nxt {
    return
  }
  
  for entry := kcp.snd_buf.next; entry != kcp.snd_buf; {
    seg := entry.val.(*Segment)
    next := entry.next
    if seg.sn >= una {
      break
    }
    kcp.snd_buf.Delete(entry)
    entry = next
  }
}

func (kcp *KCP) ack_push(sn, ts uint32) {
  kcp.acklist = append(kcp.acklist, sn, ts)
}

func (kcp *KCP) ack_get(pos uint32) (uint32, uint32) {
  return kcp.acklist[pos], kcp.acklist[pos + 1]
}

func (kcp *KCP) parse_data(seg *Segment) {
  if seg.sn < kcp.rcv_nxt || seg.sn >= kcp.rcv_wnd + kcp.rcv_nxt  {
    return
  }
  
  entry, repeat := kcp.rcv_buf.prev, false
  for ; entry != kcp.rcv_buf; entry = entry.prev {
    if entry.val.(*Segment).sn > seg.sn {
      continue
    }
    if entry.val.(*Segment).sn == seg.sn {
      repeat = true
      break
    }
    break
  }
  
  if !repeat {
    kcp.rcv_buf.After(entry, seg)
  }
  
  for entry = kcp.rcv_buf.next; entry != kcp.rcv_buf; {
    seg := entry.val.(*Segment)
    next := entry.next
    if seg.sn != kcp.rcv_nxt {
      break
    }
    
    kcp.rcv_buf.Delete(entry)
    kcp.rcv_queue.PushNode(entry)
    kcp.rcv_nxt++
    entry = next
  }
}

// rcv read received data and parse
func (kcp *KCP) input(data []byte) error {
  if data == nil || len(data) < KCP_OVERHEAD {
    return errors.New("empty data")
  }
  var una uint32
  for true {
    seg, data, err := Decode(data)
    if err != nil {
      return err
    }
    una = seg.una
    kcp.rmt_wnd = seg.wnd
    kcp.parse_una(seg.una)
    kcp.shrink_buf()
    
    switch seg.cmd {
      case KCP_CMD_ACK:
        if seg.ts < kcp.current {
          kcp.update_ack(kcp.current - seg.ts)
        }
        kcp.parse_ack(seg.sn)
        kcp.shrink_buf()
      case KCP_CMD_PUSH:
        if seg.sn > kcp.rcv_wnd + kcp.rcv_nxt {
            return errors.New("rcv buffer full")
        }
        kcp.parse_data(seg)
        kcp.ack_push(seg.sn, seg.ts)
      case KCP_CMD_WASK:
        kcp.probe |= KCP_ASK_TELL
      case KCP_CMD_WINS:
      default:
        return errors.New("unknown data command")
    }
    
    if len(data) < KCP_OVERHEAD {
      break
    }
  }
  
  if kcp.snd_una > una && kcp.cwnd < kcp.rmt_wnd {
    if kcp.cwnd < kcp.ssthresh {
      kcp.cwnd++
      kcp.incr = kcp.mss
    } else {
      if kcp.incr < kcp.mss {
        kcp.incr = kcp.mss
      }
      kcp.incr += kcp.mss * kcp.mss / kcp.incr + kcp.mss / 16
      if (kcp.cwnd + 1) * kcp.mss < kcp.incr {
        kcp.cwnd++
      }
    }
    if kcp.cwnd > kcp.rmt_wnd {
      kcp.cwnd = kcp.rmt_wnd
      kcp.incr = kcp.mss * kcp.rmt_wnd
    }
  }
  return nil
}

func (kcp *KCP) wnd_unused() uint32 {
  if kcp.rcv_queue.Len() < kcp.rcv_wnd {
    return kcp.rcv_wnd - kcp.rcv_queue.Len()
  }
  return 0
}

func (kcp *KCP) flush() error {
  if kcp.updated == 0 {
    return errors.New("updated has not been called")
  }
  
  var pos, current uint32 = 0, kcp.current
  seg := NewSegment(kcp)
  seg.cmd = KCP_CMD_ACK
  seg.una = kcp.rcv_nxt
  seg.wnd = kcp.wnd_unused()
  
  for i := 0; i < len(kcp.acklist); i += 2 {
    seg.sn, seg.ts = kcp.ack_get(uint32(i))
    if pos + KCP_OVERHEAD > kcp.mtu {
      if err := kcp.output(kcp.buffer[:pos]); err != nil {
        return err
      }
      pos = 0
    }
    seg.Encode(kcp.buffer[pos:])
    pos += KCP_OVERHEAD
  }
  kcp.acklist = []uint32{}
  
  if kcp.rmt_wnd == 0 {
    if kcp.probe_wait == 0 {
      kcp.probe_wait = KCP_PROBE_INIT
      kcp.ts_probe = current + kcp.probe_wait
    } else if current > kcp.ts_probe {
      kcp.probe_wait = max(kcp.probe_wait, KCP_PROBE_INIT)
      kcp.probe_wait += kcp.probe_wait / 2
      kcp.probe_wait = min(kcp.probe_wait, KCP_PROBE_LIMIT)
      kcp.probe |= KCP_ASK_SEND
    }
  } else {
    kcp.ts_probe = 0
    kcp.probe_wait = 0
  }
  
  if kcp.probe & KCP_ASK_TELL == 1 {
    seg.cmd = KCP_CMD_WINS
    if pos + KCP_OVERHEAD > kcp.mtu {
      if err := kcp.output(kcp.buffer[:pos]); err != nil {
        return err
      }
      pos = 0
    }
    seg.Encode(kcp.buffer[pos:])
    pos += KCP_OVERHEAD
  }
  
  if kcp.probe & KCP_ASK_SEND == 1 {
    seg.cmd = KCP_CMD_WASK
    if pos + KCP_OVERHEAD > kcp.mtu {
      if err := kcp.output(kcp.buffer[:pos]); err != nil {
        return err
      }
      pos = 0
    }
    seg.Encode(kcp.buffer[pos:])
    pos += KCP_OVERHEAD
  }
  kcp.probe = 0
  
  cwnd := min(kcp.snd_wnd, kcp.rmt_wnd)
  if kcp.nocwnd == 0 {
    cwnd = min(kcp.cwnd, cwnd)
  }
  
  for cwnd + kcp.snd_una > kcp.snd_nxt {
    entry := kcp.snd_queue.Pop()
    if entry == nil {
      break
    }
    seg := entry.val.(*Segment)
    seg.sn = kcp.snd_nxt
    seg.cmd = KCP_CMD_PUSH
    seg.rto = kcp.rx_rto
    seg.wnd = kcp.wnd_unused()
    seg.resendts = current
    seg.ts = current
    seg.una = kcp.rcv_nxt
    kcp.snd_nxt++
    
    kcp.snd_buf.PushNode(entry)
  }
  
  var resent, rtomin uint32
  if kcp.faskresend > 0 {
    resent = kcp.faskresend
  } else {
    resent = 0xffffffff
  }
  
  if kcp.nodelay == 0 {
    rtomin = kcp.rx_rto >> 3
  } else {
    rtomin = 0
  }
  
  send, lost, change := false, false, false
  for entry := kcp.snd_buf.next; entry != kcp.snd_buf; entry = entry.next {
    seg := entry.val.(*Segment)
    send = false
    if seg.xmit == 0 {
      seg.xmit++
      seg.rto = kcp.rx_rto
      seg.resendts = current + seg.rto + rtomin
      send = true
    } else if seg.resendts < current {
      seg.xmit++
      if kcp.nodelay == 0 {
        seg.rto += kcp.rx_rto
      } else {
        seg.rto += kcp.rx_rto / 2
      }
      seg.resendts = current + seg.rto
      lost, send = true, true
    } else if seg.fastack >= kcp.faskresend {
      seg.xmit++
      seg.fastack = 0
      seg.resendts = current + seg.rto
      send, change = true, true
    }
    
    if !send {
      continue
    }
    
    if pos + seg.len + KCP_OVERHEAD > kcp.mtu {
      if err := kcp.output(kcp.buffer[:pos]); err != nil {
        return err
      }
      pos = 0
    }
    seg.wnd = kcp.wnd_unused()
    seg.una = kcp.rcv_nxt
    seg.ts = current
    seg.Encode(kcp.buffer[pos:])
    pos += KCP_OVERHEAD + seg.len
    
    if seg.xmit >= kcp.dead_link {
      kcp.state = 0
    }
  }
  
  // flushing remaining data
  if pos != 0 {
    if err := kcp.output(kcp.buffer[:pos]); err != nil {
      return err
    }
    pos = 0
  }
  
  // calculating congestion window
  if change {
    inflight := kcp.snd_nxt - kcp.snd_una
    kcp.ssthresh = max(inflight / 2, KCP_THRESH_MIN)
    kcp.cwnd = kcp.ssthresh + resent
    kcp.incr = kcp.cwnd * kcp.mss
  } else if lost {
    kcp.ssthresh = max(kcp.cwnd / 2, KCP_THRESH_MIN)
    kcp.cwnd = 1
    kcp.incr = kcp.mss
  }
  
  if kcp.cwnd < 1 {
    kcp.cwnd = 1
    kcp.incr = kcp.mss
  }
  return nil
}

func (kcp *KCP) update(current uint32) error {
  kcp.current = current
  if kcp.updated == 0 {
    kcp.updated = 1
    kcp.ts_flush = kcp.current + kcp.interval
  }
  
  slap := timediff(kcp.current, kcp.ts_flush)
  if slap > 10000 || slap < -10000 {
    kcp.ts_flush = kcp.current
    slap = 0
  }
  
  kcp.ts_flush += kcp.interval
  if kcp.ts_flush < kcp.current {
    kcp.ts_flush = kcp.current + kcp.interval
  }
  return kcp.flush()
}

func (kcp *KCP) check(current uint32) uint32 {
  if kcp.updated == 0 {
    return current
  }
  
  slap := timediff(current, kcp.ts_flush)
  if slap > 10000 || slap < 0 {
    return current
  }
  
  var recent uint32
  for entry := kcp.snd_buf.next; entry != kcp.snd_buf; entry = entry.next {
    seg := entry.val.(*Segment)
    if seg.resendts < current {
      return current
    }
    
    if recent == 0 {
      recent = seg.resendts
    } else if recent < seg.resendts {
      recent = seg.resendts
    }
  }
  rslt := min(recent, kcp.ts_flush)
  return min(rslt, current + kcp.interval)
}

func (kcp *KCP) setmtu(mtu uint32) error {
  if mtu < KCP_OVERHEAD || mtu < 50 {
    return errors.New("mtu too small")
  }
  
  buffer := make([]byte, mtu)
  kcp.mtu = mtu
  kcp.mss = mtu - KCP_OVERHEAD
  kcp.buffer = buffer
  return nil
}

func (kcp *KCP) set_interval(interval uint32) {
  if interval > 5000 {
    interval = 5000
  } else if interval < 10 {
    interval = 10
  }
  kcp.interval = interval
}

func (kcp *KCP) set_nodelay(nodelay, interval, resend, nc uint32) {
  kcp.set_interval(interval)
  if nodelay >= 0 {
    kcp.nodelay = nodelay
    if nodelay == 0 {
      kcp.rx_minrto = KCP_RTO_MIN
    } else {
      kcp.rx_minrto = KCP_RTO_NDL
    }
  }
  
  if resend >= 0 {
    kcp.faskresend = resend
  }
  
  if nc >= 0 {
    kcp.nocwnd = nc
  }
}

func (kcp *KCP) wnd_size(rcv_wnd, snd_wnd uint32) {
  if rcv_wnd > 0 {
    kcp.rcv_wnd = rcv_wnd
  }
  
  if snd_wnd > 0 {
    kcp.snd_wnd = snd_wnd
  }
}