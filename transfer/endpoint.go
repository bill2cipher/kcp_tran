package transfer

import (
  "os"
  "log"
  "time"
  "path"
  "bytes"
  "errors"
  "encoding/binary"
)

import (
  "github.com/golang/protobuf/proto"
)

import (
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
  sock    Pipe `desc:"kcp socket used to recv/send data"`
  name    string `desc:"file being written"`
  
  ask_idx []uint32 `desc:"index need to be sent"`
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

func (end *EndPoint) Encode() []byte {
  var buffer bytes.Buffer
  var store = make([]byte, 4)
  log.Printf("encode %d %d %d", end.total, end.block, end.cnt)
  binary.LittleEndian.PutUint32(store, end.total)
  buffer.Write(store)
  
  binary.LittleEndian.PutUint32(store, end.block)
  buffer.Write(store)
  
  binary.LittleEndian.PutUint32(store, end.cnt)
  buffer.Write(store)
  
  temp := make([]bool, end.cnt)
  for _, idx := range end.ask_idx {
    temp[idx] = true
  }

  for i := uint32(0); i < end.cnt; i++ {
    if temp[i] {
      continue
    }
    binary.LittleEndian.PutUint32(store, i)
    buffer.Write(store)
  }
  return buffer.Bytes()
}

func (end *EndPoint) Decode(data []byte) error {
  if len(data) < 3 * 4 {
    return errors.New("data size too small")
  }
  end.total = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  end.block = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  end.cnt = binary.LittleEndian.Uint32(data)
  data = data[4:]
  
  temp := make([]bool, end.cnt)
  log.Printf(" %d %d %d", end.total, end.block, end.cnt)
  for len(data) > 4 {
    val := binary.LittleEndian.Uint32(data)
    temp[val] = true
    data = data[4:]
  }
  for i := uint32(0); i < uint32(len(temp)); i++ {
    if temp[i] {
      continue
    }
    end.ask_idx = append(end.ask_idx, i)
  }
  return nil
}

func (end *EndPoint) SetInfo(total, block, cnt uint32) {
  end.total, end.block, end.cnt = total, block, cnt
  end.ask_idx = make([]uint32, end.cnt)
  for i := uint32(0); i < end.cnt; i++ {
    end.ask_idx[i] = i
  }
}

func (end *EndPoint) AskIndex(size int) []uint32 {
  if len(end.ask_idx) > size {
    rslt := end.ask_idx[:size]
    end.ask_idx = end.ask_idx[size:]
    return rslt
  } else {
    rslt := end.ask_idx
    end.ask_idx = nil
    return rslt
  }
}

func (end *EndPoint) SendInit(info os.FileInfo, timeout time.Duration) error {
  mesg := new(msg.Transfer)
  init := new(msg.SendInit)
  size, block := uint32(info.Size()), uint32(BLOCK_SIZE)
  cnt, name := (size + block - 1) / block, path.Base(info.Name())
  mesg.SendInit = init
  init.Total, init.Block = &size, &block
  init.Cnt, init.Name = &cnt, &name
  
  end.total, end.block = size, block
  end.cnt, end.name = cnt, name
  if body, err := EncodeMesg(mesg); err != nil {
    return err
  } else {
    return end.sock.Write(body)
  }
}

func (end *EndPoint) AskPartial(index []uint32) error {
  mesg, ask := new(msg.Transfer), new(msg.AskPartial)
  mesg.AskPartial, ask.Index = ask, index
  if body, err := EncodeMesg(mesg); err != nil {
    return err
  } else {
    return end.sock.Write(body)
  }
}

func (end *EndPoint) RecvInit(name string, total, block, cnt uint32) error {
  name = path.Base(name)
  mesg, init := new(msg.Transfer), new(msg.RecvInit)
  mesg.RecvInit, init.Name = init, &name
  if total != 0 && block != 0 && cnt != 0 {
  } else {
    total, block, cnt = 0, 0, 0
  }

  init.Total, init.Block, init.Cnt = &total, &block, &cnt
  if body, err := EncodeMesg(mesg); err != nil {
    return err
  } else {
    return end.sock.Write(body)
  }
}

func (end *EndPoint) Error(code uint32) error {
  mesg, status := new(msg.Transfer), new(msg.Error)
  mesg.Status, status.Code = status, &code
  if body, err := EncodeMesg(mesg); err != nil {
    return err
  } else {
    return end.sock.Write(body)
  }
}

func (end *EndPoint) SendPartial(index uint32, data []byte) error {
  mesg, partial := new(msg.Transfer), new(msg.SendPartial)
  mesg.SendPartial = partial
  partial.Index = &index
  partial.Data = data
  partial.Checksum = CheckSum(data)
  if body, err := EncodeMesg(mesg); err != nil {
    return err
  } else {
    return end.sock.Write(body)
  }
}

func (end *EndPoint) ReadMessageTimeout(timeout time.Duration) (*msg.Transfer, error) {
  signal := make(chan []byte)
  go read_proc(end.sock, signal)
  defer close(signal)
  
  var (
    mesg []byte
    err  error
  )

  if timeout == 0 {
    if mesg = <- signal; mesg == nil {
      err = errors.New("read message failed")
    }
  } else {
    select {
    case mesg = <- signal:
      if mesg == nil {
        err = errors.New("read message failed")
      }
    case <- time.After(timeout):
      err = errors.New("read message timeout")
    }
  }
  
  if err != nil {
    return nil, err
  }

  rslt := new(msg.Transfer)
  if err = proto.Unmarshal(mesg, rslt); err != nil {
    return nil, err
  } else {
    return rslt, nil
  }
}


func read_proc(sock Pipe, signal chan []byte) {
  defer recover()
  var rslt []byte
  header := make([]byte, 4)
  if dlen, err := ReadHeader(header, sock); err != nil {
    log.Printf("read message header failed %s", err.Error())
  } else if mesg, err := ReadMessage(dlen, sock); err != nil {
    log.Printf("read message body failed %s", err.Error())
  } else {
    rslt = mesg
  }
  select{
  case signal <- rslt:
  case <-time.After(time.Second):
  }
}
