package kcp

import (
  "bytes"
  "errors"
  "encoding/binary"
)

type Segment struct {
  conv uint32 `desc:"id sepecifying conversasion id"`
  seq  uint32 `desc:"id specifying"`
  cmd  uint32 `desc:"command within this segment"`
  data []byte `desc:"segment data"`
}

func (seg *Segment) Encode() []byte {
  var buffer bytes.Buffer
  var store = make([]byte, 4)
  
  binary.LittleEndian.PutUint32(store, seg.conv)
  buffer.Write(store)
  
  binary.LittleEndian.PutUint32(store, seg.seq)
  buffer.Write(store)
  
  binary.LittleEndian.PutUint32(store, seg.cmd)
  buffer.Write(store)
  
  dlen := uint32(len(seg.data))
  binary.LittleEndian.PutUint32(store, dlen)
  buffer.Write(store)
  buffer.Write(seg.data)
  
  return buffer.Bytes()
}

func (seg *Segment) Decode(data []byte) error {
  seg.conv = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.seq = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  seg.cmd = binary.LittleEndian.Uint32(data)
  data = data[4:]

  dlen := binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  if uint32(len(data)) < dlen {
    return errors.New("content format error: data len too large")
  } else {
    seg.data = data[:dlen]
  }
  return nil
}
