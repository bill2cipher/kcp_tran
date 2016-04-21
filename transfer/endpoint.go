package transfer

import (
  "os"
  "log"
  "time"
  "sync"
  "errors"
  "crypto/md5"
  "encoding/binary"
)

import (
  "github.com/golang/protobuf/proto"
)

import (
  "github.com/jellybean4/kcp_tran/kcp"
  "github.com/jellybean4/kcp_tran/msg"
)

type Pipe interface {
  Write(data []byte) error
  Read(store []byte) (int, error)
}

type EndPoint struct {
  id      uint32 `desc:"identifier of this EndPoint"`
  total   uint32 `desc:"total size of the tranfered data"`
  block   uint32 `desc:"block size of the split data"`
  cnt     uint32 `desc:"how many blocks will be transfered"`
  current uint32 `desc:"how many blocks have been written"`
  rcved   []*msg.SendPartial `desc:"where recvd and verified block is stored"`
  snd_buf []*msg.SendPartial `desc:"where data need to be sent"`
  sent    []*msg.SendPartial `desc:"where data sent but not ack"`
  sock    Pipe `desc:"kcp socket used to recv/send data"`
  name    string `desc:"file being written"`
}

func NewEndPoint(id uint32, sock Pipe) *EndPoint {
  end := new(EndPoint)
  end.init(id, sock)
  return end
}

func (end *EndPoint) init(id uint32, sock Pipe) {
  end.id = id
  end.sock = sock
}

func (end *EndPoint) sockout() {
  for !end.close {
    data := <- end.sockflow
    end.sock.Write(data)
  }
}

func (end *EndPoint) outflow() {
  sequence, start, over := uint32(0), uint32(0), uint32(0)
  trans, partial := new(msg.Transfer), new(msg.SendPartial)
  head := make([]byte, 8)
  for !end.close {
    if len(end.snd_buf) == 0 {
      data := <- end.flow
      dlen := uint32(len(data))
      over = start + dlen
      trans.SendPartial = partial
      partial.Checksum = CheckSum(data)
      partial.Sequence = &sequence
      partial.Size = &dlen
      partial.Start = &start
      partial.End = &over
      partial.Content = data
      sequence++
    } else {
      end.mutex.Lock()
      partial = end.snd_buf[0]
      end.snd_buf = end.snd_buf[1:]
      end.mutex.Unlock()
      trans.SendPartial = partial
    }
    if partial.Content == nil {
      return
    }

    if encoded, err := proto.Marshal(trans); err != nil {
      log.Printf("encode msg failed %s", err.Error())
      return
    } else {
      dlen := uint32(len(encoded))
      binary.LittleEndian.PutUint32(head, dlen)
      binary.LittleEndian.PutUint32(head[4:], 1)
      end.sockflow <- append(head, encoded...)
      end.mutex.Lock()
      end.snd_buf = append(end.snd_buf, partial)
      end.mutex.Unlock()
    }
    start, over = over, 0
  }
}

func (end *EndPoint) flush() {
  for !end.close {
    for len(end.rcved) <= 0 {
      select {
      case <- end.arrived:
      case <- time.After(time.Second):
      }
    }
    var partial []*msg.SendPartial
    end.mutex.Lock()
    partial = end.rcved
    end.rcved = nil
    end.mutex.Unlock()
    if len(partial) == 0 {
      continue
    }
    var rewrite []*msg.SendPartial
    for i := 0; i < len(partial); i++ {
      if err := end.write(partial[i]); err != nil {
        log.Printf("write partial %d failed %s", partial[i].Sequence, err.Error())
        rewrite = append(rewrite, partial[i])
      } else {
        end.current++
      }
    }
    
    if len(rewrite) != 0 {
      end.mutex.Lock()
      end.rcved = append(end.rcved, rewrite...)
      end.mutex.Unlock()
    }
  }
  if len(end.rcved) == 0 {
    return
  }
  for i := 0; i < len(end.rcved); i++ {
    if err := end.write(end.rcved[i]); err != nil {
      log.Printf("write file %s failed %s", end.name, err.Error())
      return
    }
  }
  end.wfile.Close()
}

func (end *EndPoint) write(partial *msg.SendPartial) error {
  if _, err := end.wfile.Seek(int64(*partial.Start), os.SEEK_SET); err != nil {
    return err
  }
  if cnt, err := end.wfile.Write(partial.Content); err != nil {
    return err
  } else if uint32(cnt) < *partial.Size {
    return errors.New("write partial size not enough")
  }
  return nil
}

func (end *EndPoint) demon() {
  store := make([]byte, 8)
  for !end.close {
    if dlen, code, err := end.readLen(store); err != nil {
      log.Printf("read msg len failed %s", err.Error())
      continue
    } else if data, err := end.readMsg(dlen); err != nil {
      log.Printf("read msg failed %d %s", dlen, err.Error())
      continue
    } else if code == 1 {
      trans := new(msg.Transfer)
      if err := proto.Unmarshal(data, trans); err != nil {
        continue
      }
      end.execute(trans)
    } else {
      trans := new(msg.TransferRp)
      if err := proto.Unmarshal(data, trans); err != nil {
        continue
      }
      end.ending(trans)
    }
  }
}

func (end *EndPoint) ending(trans *msg.TransferRp) {
  switch {
  case trans.SendInitRp != nil:
    end.endSendInit(trans.SendInitRp)
  case trans.SendPartialRp != nil:
    end.endSendPartial(trans.SendPartialRp)
  case trans.SendFinishRp != nil:
    end.endSendFinish(trans.SendFinishRp)
  case trans.RecvInitRp != nil:
    end.endRecvInit(trans.RecvInitRp)
  }
}

func (end *EndPoint) endSendInit(init *msg.SendInitRp) {
  go end.outflow()
}

func (end *EndPoint) endSendPartial(partial *msg.SendPartialRp) {
  if *partial.Status != 0 {
    log.Printf("rcv data %d failed", *partial.Sequence)
    return
  }
  end.mutex.Lock()
  var rslt []*msg.SendPartial
  for idx, entry := range end.snd_buf {
    if *entry.Sequence < *partial.Sequence {
      rslt = append(rslt, entry)
    } else if *entry.Sequence == *partial.Sequence {
      
    } else {
      rslt = append(rslt, end.snd_buf[idx:]...)
    }
  }
  end.snd_buf = rslt
  end.mutex.Unlock()
}

func (end *EndPoint) endSendFinish(finish *msg.SendFinishRp) {
  if *finish.Status != 0 {
    log.Printf("finish send failed")
  }
  end.close = true
}

func (end *EndPoint) endRecvInit(init *msg.RecvInitRp) {
  if *init.Status != 0 {
    end.close = true
    log.Printf("rcv init failed %d", *init.Status)
  }
}

func (end *EndPoint) execute(trans *msg.Transfer) {
  var rply []byte
  switch {
  case trans.SendInit != nil:
    rply = end.sendInit(trans.SendInit)
  case trans.SendPartial != nil:
    rply = end.sendPartial(trans.SendPartial)
  case trans.SendFinish != nil:
    rply = end.sendFinish(trans.SendFinish)
  case trans.RecvInit != nil:
    rply = end.recvInit(trans.RecvInit)
  }
  if rply == nil {
    return
  }
  head := make([]byte, 8)
  dlen := uint32(len(rply))
  binary.LittleEndian.PutUint32(head, dlen)
  binary.LittleEndian.PutUint32(head[4:], 2)
  end.sockflow <- append(head, rply...)
}

func (end *EndPoint) sendInit(init *msg.SendInit) []byte {
  end.block = *init.Block
  end.cnt = *init.Cnt
  end.name = *init.Name
  end.total = *init.Total
  reply, initRp := new(msg.TransferRp), new(msg.SendInitRp)
  status := uint32(0)
  reply.SendInitRp = initRp
  os.Remove(end.name)

  if file, err := os.OpenFile(end.name, os.O_APPEND | os.O_WRONLY | os.O_CREATE, 0660); err != nil {
    status = 1
    log.Printf("send init failed %s", err.Error())
  } else if err := FillFile(file, end.total); err != nil {
    status = 2
    log.Printf("fill file size %d failed %s", end.total, err.Error())
  } else {
    status = 0
    end.wfile = file
    go end.flush()
  }
  initRp.Status = &status
  if encode, err := proto.Marshal(reply); err != nil {
    return nil
  } else {
    return encode
  }
}

func (end *EndPoint) sendPartial(partial *msg.SendPartial) []byte {
  block := partial
  reply, partialRp := new(msg.TransferRp), new(msg.SendPartialRp)
  reply.SendPartialRp = partialRp
  status, sequence := uint32(0), *partial.Sequence
  if calc := CheckSum(partial.Content); BinaryCompare(calc, partial.Checksum) != 0 {
    status = 1
  } else {
    status = 0
  }
  end.mutex.Lock()
  end.rcved = append(end.rcved, block)
  end.mutex.Unlock()
  select {
  case end.arrived <- true:
  default:
  }
  partialRp.Status = &status
  partialRp.Sequence = &sequence
  if encode, err := proto.Marshal(reply); err != nil {
    return nil
  } else {
    return encode
  }
}

func (end *EndPoint) sendFinish(finish *msg.SendFinish) []byte {
  reply, finishRp := new(msg.TransferRp), new(msg.SendFinishRp)
  reply.SendFinishRp = finishRp
  status := uint32(0)
  finishRp.Status = &status
  end.close = true
  if encode, err := proto.Marshal(reply); err != nil {
    return nil
  } else {
    return encode
  }
}

func (end *EndPoint) recvInit(init *msg.RecvInit) []byte {
  end.name = *init.Name
  var rslt proto.Message
  if info, err := os.Stat(end.name); err == nil || os.IsExist(err) {
    end.total = uint32(info.Size())
    end.block = BLOCK_SIZE
    end.cnt = (end.total + end.block - 1) / end.block
    if file, err := os.OpenFile(end.name, os.O_RDONLY, 0); err == nil {
      end.rfile = file
      go end.readfile()
      reply, send := new(msg.Transfer), new(msg.SendInit)
      reply.SendInit = send
      send.Total = &end.total
      send.Block = &end.block
      send.Cnt = &end.cnt
      send.Name = &end.name
      rslt = reply
    } else {
      reply, recv := new(msg.TransferRp), new(msg.RecvInitRp)
      reply.RecvInitRp = recv
      status := uint32(1)
      recv.Status = &status
      rslt = reply
    }
  } else {
    reply, recv := new(msg.TransferRp), new(msg.RecvInitRp)
    reply.RecvInitRp = recv
    status := uint32(2)
    recv.Status = &status
    rslt = reply
  }
  if encode, err := proto.Marshal(rslt); err != nil {
    return nil
  } else {
    return encode
  }
}

func (end *EndPoint) readLen(store []byte) (uint32, uint32, error) {
  if err := end.readData(store); err != nil {
    return 0, 0, err
  } else {
    dlen := binary.LittleEndian.Uint32(store)
    code := binary.LittleEndian.Uint32(store[4:])
    return dlen, code, nil
  }
}

func (end *EndPoint) readMsg(dlen uint32) ([]byte, error) {
  buffer := make([]byte, dlen)
  if err := end.readData(buffer); err != nil {
    return nil, nil
  } else {
    return buffer, nil
  }
}

func (end *EndPoint) readData(store []byte) error {
  size := 0
  for size < len(store) {
    if cnt, err := end.sock.Read(store[size:]); err != nil {
      return nil
    } else {
      size += cnt
    }
  }
  return nil
}

func CheckSum(data []byte) []byte {
  rslt := md5.Sum(data)
  return rslt[:]
}

func BinaryCompare(a, b []byte) int {
  var mlen int
  if len(a) > len(b) {
    mlen = len(b)
  } else {
    mlen = len(a)
  }
  for i := 0; i < mlen; i++ {
    if a[i] > b[i] {
      return 1
    } else if a[i] < b[i] {
      return -1
    }
  }
  
  if len(a) > len(b) {
    return 1
  } else if len(a) < len(b) {
    return -1
  } else {
    return 0
  }
}

func FillFile(file *os.File, total uint32) error {
  cnt := total / 512
  left := total - 512 * cnt
  buffer := make([]byte, 512)
  for i := uint32(0); i < cnt; i++ {
    if _, err := file.Write(buffer); err != nil {
      return err
    }
  }
  if left != 0 {
    if _, err := file.Write(buffer[:left]); err != nil {
      return err
    }
  }
  return nil
}
