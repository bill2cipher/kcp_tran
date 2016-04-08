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
  kCP_THRESH_MIN = 2  
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

func (kcp *KCP) init(conv uint32, writer io.Writer) {
  kcp.conv = conv
  kcp.writer = writer
  kcp.snd_wnd, kcp.rcv_wnd, kcp.rmt_wnd = KCP_WND_SND, KCP_WND_RCV, KCP_WND_RCV
  kcp.mtu = KCP_MTU_DEF
  kcp.mss = KCP_MTU_DEF - KCP_OVERHEAD
  
  kcp.buffer = make([]byte, kcp.mtu + KCP_OVERHEAD)
  kcp.snd_queue, kcp.snd_buf = NewQueue(), NewQueue()
  kcp.rcv_queue, kcp.rcv_buf = NewQueue(), NewQueue()
  
  kcp.rx_rto, kcp.rx_minrto = KCP_RTO_DEF, KCP_RTO_MIN
  kcp.interval, kcp.ts_flush = KCP_INTERVAL, KCP_INTERVAL
  kcp.dead_link = KCP_DEADLINK
}

func (kcp *KCP) output(data []byte) (int, error) {
  if len(data) == 0 {
    return 0, nil
  }
  return kcp.writer.Write(data)
}

func (kcp *KCP) receive(peek bool) ([]byte, error) {
  peeksize, err := kcp.peeksize()
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
  
  var rslt uint32 = 0
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
    break
  }
  
  for entry := kcp.rcv_buf.prev; entry != kcp.rcv_buf; entry = entry.prev {
    if entry.val.(*Segment).sn > seg.sn {
      continue
    }
    if entry.val.(*Segment).sn == seg.sn {
      break
    }
    entry.Insert(seg)
  }
  
  // move continuation data from rcv_buf into rcv_queue
  for entry := kcp.rcv_buf.next; entry != kcp.rcv_buf; {
    seg := entry.val.(*Segment)
    next := entry.next
    if seg.sn != kcp.rcv_nxt {
      break
    }
    
    kcp.rcv_queue.PushNode(entry)
    kcp.rcv_buf.Delete(entry)
    kcp.rcv_nxt++
  }
}


// rcv read received data and parse
func (kcp *KCP) input(data []byte) error {
  if data == nil || len(data) < KCP_OVERHEAD {
    return errors.New("empty data")
  }
  
  for true {
    seg, data, err := Decode(data)
    if err != nil {
      return err
    }
    
    kcp.rmt_wnd = seg.wnd
    kcp.parse_una(seg.una)
    kcp.shrink_buf()
    
    switch seg.cmd {
      case KCP_CMD_PUSH:
        if seg.sn > kcp.rcv_wnd + kcp.rcv_nxt
        
    }
    
    if len(data) < KCP_OVERHEAD {
      break
    }
  }
  return nil
}