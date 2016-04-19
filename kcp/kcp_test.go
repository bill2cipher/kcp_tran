package kcp

import (
  "log"
  "time"
  "math/rand"
  "testing"
	"encoding/binary"
)



type DelayPkg struct {
  ts uint32
  size int
  data []byte
}

type LatencySimulator struct {
  current uint32
  lostrate, rttmin, rttmax int
	nmax int
  s2c []*DelayPkg
  c2s []*DelayPkg
  srand *rand.Rand
  crand *rand.Rand
}

func (sim *LatencySimulator) init(lostrate, rttmin, rttmax, nmax int) {
  sim.current = clock()
  sim.nmax = nmax
  sim.lostrate, sim.rttmin, sim.rttmax = lostrate, rttmin / 2, rttmax / 2
  
  ssrc := rand.NewSource(98123789243423)
  csrc := rand.NewSource(92728239812387)
  sim.srand = rand.New(ssrc)
  sim.crand = rand.New(csrc)
}

func (sim *LatencySimulator) send(sender int, data []byte) {
  if sender == 1 {
    if sim.lostrate > (sim.srand.Int() % 100) {
      return
    }
    if len(sim.s2c) > sim.nmax {
      return
    }
  } else {
    if sim.lostrate > (sim.crand.Int() % 100) {
      return
    }
    if len(sim.c2s) > sim.nmax {
      return
    }
  }
  
  sim.current = clock()
  pkg := new(DelayPkg)
  pkg.size = len(data)
  pkg.data = append(pkg.data, data...)
  rtt := sim.rttmin + int(float32(sim.rttmax - sim.rttmin) * rand.Float32()) 
  pkg.ts = uint32(rtt) + sim.current
  
  if sender == 1 {
    sim.s2c = sim.push(sim.s2c, pkg)
  } else {
    sim.c2s = sim.push(sim.c2s, pkg)
  }
}

func (sim *LatencySimulator) receive(receiver int) []byte {
  var pkg *DelayPkg
  sim.current = clock()
  if receiver == 1 {
    if len(sim.c2s) == 0 {
      return nil
    }
    if pkg = sim.c2s[0]; pkg.ts > sim.current {
      return nil
    }
    sim.c2s = sim.c2s[1:]
  } else {
    if len(sim.s2c) == 0 {
      return nil
    }
    if pkg = sim.s2c[0]; pkg.ts > sim.current {
      return nil
    }
    sim.s2c = sim.s2c[1:]
  }
  var copy []byte
  return append(copy, pkg.data...)
}

func (sim *LatencySimulator) push(queue []*DelayPkg, pkg *DelayPkg) []*DelayPkg {
  var n []*DelayPkg
  var i int
  for i = 0; i < len(queue); i++ {
    if queue[i].ts <= pkg.ts {
      n = append(n, queue[i])
      continue
    }
    break
  }
  n = append(n, pkg)
  n = append(n, queue[i:]...)
  return n
}

func aTestKcp(t *testing.T) {
  test(0, t)
  test(1, t)
  test(2, t)
}

type UDP struct {
  vnet *LatencySimulator
  mode int
}

func (udp *UDP) Write(data []byte) (int, error) {
  udp.vnet.send(udp.mode, data)
  return len(data), nil
}

func test(mode int, t *testing.T) {
  vnet := new(LatencySimulator)
  vnet.init(10, 60, 120, 1000)
  sudp := new(UDP)
  sudp.vnet = vnet
  sudp.mode = 1
  skcp := NewKCP(123456, sudp.Write)
  
  cudp := new(UDP)
  cudp.vnet = vnet
  cudp.mode = 0
  ckcp := NewKCP(123456, cudp.Write)
  
  skcp.wnd_size(128, 128)
  ckcp.wnd_size(128, 128)
  
  current := clock()
  slap := current + 20
  
  if mode == 0 {
    skcp.set_nodelay(0, 10, 0, 0)
    ckcp.set_nodelay(0, 10, 0, 0)
  } else if mode == 1 {
    skcp.set_nodelay(0, 10, 0, 1)
    ckcp.set_nodelay(0, 10, 0, 1)
  } else {
    skcp.set_nodelay(1, 10, 2, 1)
    ckcp.set_nodelay(1, 10, 2, 1)
  }
  
  index, next, cnt := uint32(0), uint32(0), uint32(0)
  for true {
    
    time.Sleep(1 * time.Millisecond)
    current = clock()
    skcp.update(current)
    ckcp.update(current)
    
    for ; slap <= current; slap += 20 {
      buffer := make([]byte, 8)
      binary.LittleEndian.PutUint32(buffer, index)
      binary.LittleEndian.PutUint32(buffer[4:], current)
      index++
      skcp.send(buffer)
    }
    
    if cnt + 100 < index {
      skcp.debug = true
      ckcp.debug = true
      log.Println("catch it")
    }
    
    for true {
      if rcv := vnet.receive(2); rcv != nil {
        ckcp.input(rcv)
      } else {
        break
      }
    }
    
    for true {
      if rcv := vnet.receive(1); rcv != nil {
        skcp.input(rcv)
      } else {
        break
      }
    }
    
    for true {
      if data, err := ckcp.receive(false); err != nil {
        break
      } else {
        ckcp.send(data)
      }
    }
    for true {
      if data, err := skcp.receive(false); err != nil {
        break
      } else {
        idx := binary.LittleEndian.Uint32(data)
        //ts  := binary.LittleEndian.Uint32(data[4:])
        if idx != next {
          log.Printf("receive pkg out of order %d %d %d", idx, next, index)
        }
        log.Printf("rcv idx : %d", idx)
        next++
        cnt = index
      }
    }
    if next >= 1000 {
      break
    }
  }
}
