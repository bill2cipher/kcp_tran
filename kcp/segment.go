package kcp

import (
  "errors"
  "encoding/binary"
	"bytes"
	"fmt"
)

type Segment struct {
  conv uint32
  cmd  uint32
  frg  uint32
  wnd  uint32
  ts   uint32
  sn   uint32
  una  uint32
  len  uint32
  rto  uint32
  xmit uint32
  data []byte
  fastack  uint32
  resendts uint32 
}

func NewSegment(kcp *KCP) *Segment {
  seg := new (Segment)
  seg.init(kcp)
  return seg
}

func (seg *Segment) init(kcp *KCP) {
  seg.conv = kcp.conv
}

func (seg *Segment) Encode(buffer []byte) {
  binary.LittleEndian.PutUint32(buffer, seg.conv)
  buffer = buffer[4:]
  
  binary.LittleEndian.PutUint32(buffer, seg.sn)
  buffer = buffer[4:]
  
  binary.LittleEndian.PutUint32(buffer, seg.frg)
  buffer = buffer[4:]
  
  binary.LittleEndian.PutUint32(buffer, seg.cmd)
  buffer = buffer[4:]
  
  binary.LittleEndian.PutUint32(buffer, seg.una)
  buffer = buffer[4:]
  
  binary.LittleEndian.PutUint32(buffer, seg.wnd)
  buffer = buffer[4:]
  
  binary.LittleEndian.PutUint32(buffer, seg.ts)
  buffer = buffer[4:]
  
  dlen := uint32(len(seg.data))
  binary.LittleEndian.PutUint32(buffer, dlen)
  buffer = buffer[4:]

  for i := 0; i < len(seg.data); i++ {
    buffer[i] = seg.data[i]
  }
}

func Decode(data []byte) (*Segment, []byte, error) {
  seg := new(Segment)
  
  seg.conv = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.sn = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.frg = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.cmd = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.una = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.wnd = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.ts = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.len = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  if uint32(len(data)) < seg.len {
    msg := fmt.Sprintf("content format error: data len too large %d/%d", len(data), seg.len)
    return nil, nil, errors.New(msg)
  } else {
    seg.data = data[:seg.len]
  }
  return seg, data[seg.len:], nil
}


func (seg *Segment) dump() string {
  var buffer bytes.Buffer
  buffer.WriteString(fmt.Sprintf("cmd : %d\n", seg.cmd))
  buffer.WriteString(fmt.Sprintf("una : %d\n", seg.una))
  buffer.WriteString(fmt.Sprintf("sn  : %d\n", seg.sn))
  buffer.WriteString(fmt.Sprintf("frg : %d\n", seg.frg))
  buffer.WriteString(fmt.Sprintf("wnd : %d\n", seg.wnd))
  buffer.WriteString(fmt.Sprintf("len: %d\n", seg.len))
  return buffer.String()
}
