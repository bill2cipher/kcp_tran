package transfer

import (
  "os"
  "log"
  "bytes"
  "errors"
  "sync"
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

const (
  BLOCK_SIZE = 1024 * 1024 * 4
)

type EndPoint struct {
  id      uint32 `desc:"identifier of this EndPoint"`
  total   uint32 `desc:"total size of the tranfered data"`
  block   uint32 `desc:"block size of the split data"`
  cnt     uint32 `desc:"how many blocks will be transfered"`
  rcved   []*msg.SendPartial `desc:"where recvd and verified block is stored"`
  snd_buf []*msg.SendPartial `desc:"where data need to be sent"`
  sent    []*msg.SendPartial `desc:"where data sent but not ack"`
  sock    *kcp.KDP `desc:"kcp socket used to recv/send data"`
  name    string `desc:"file being written"`
  file    *os.File `desc:"file struct used to write/read data"`
  close   bool `desc:"if this end point is closed"`
  arrived chan bool
  mutex   *sync.Mutex
}

func NewEndPoint(id uint32, sock *kcp.KDP) *EndPoint {
  end := new(EndPoint)
  end.init(id, sock)
  go end.demon()
  go end.flush()
  return end
}

func (end *EndPoint) init(id uint32, sock *kcp.KDP) {
  end.id = id
  end.sock = sock
  end.arrived = make(chan bool)
  end.mutex = new(sync.Mutex)
}

func (end *EndPoint) flush() {
  for !end.close {
    select{
    case <- end.arrived:
    default:
      continue
    }
    var partial []*msg.SendPartial
    end.mutex.Lock()
    partial = end.rcved
    end.rcved = nil
    end.mutex.Unlock()
    if partial == nil {
      continue
    } else if err := end.write(partial); err != nil {
      end.mutex.Lock()
      end.rcved = append(end.rcved)
      end.mutex.Unlock()
    }
  }
}

func (end *EndPoint) write(partial *msg.SendPartial) error {
  if _, err := end.file.Seek(int64(*partial.Start), os.SEEK_SET); err != nil {
    return err
  }
  if cnt, err := end.file.Write(partial.Content); err != nil {
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

}

func (end *EndPoint) execute(trans *msg.Transfer) error {
  switch {
  case trans.SendInit != nil:
    return end.sendInit(trans.SendInit)
  case trans.SendPartial != nil:
    return end.sendPartial(trans.SendPartial)
  case trans.SendFinish != nil:
    return end.sendFinish(trans.SendFinish)
  case trans.RecvInit != nil:
    return end.recvInit(trans.RecvInit)
  default:
    return errors.New("unknown command")
  }
}

func (end *EndPoint) sendInit(init *msg.SendInit) error {
  end.block = *init.Block
  end.cnt = *init.Cnt
  end.name = *init.Name
  end.total = *init.Total
  os.Remove(end.name)
  if file, err := os.OpenFile(end.name, os.O_APPEND | os.O_WRONLY | os.O_CREATE, 0660); err != nil {
    return err
  } else {
    end.file = file
  }
  return nil
}

func (end *EndPoint) sendPartial(partial *msg.SendPartial) error {
  block := partial
  if calc := CheckSum(partial.Content); BinaryCompare(calc, partial.Checksum) != 0 {
    return errors.New("partial check failed")
  }
  end.rcv_buf = append(end.rcv_buf, block)
  select {
  case end.arrived <- true:
  default:
  }
  return nil
}

func (end *EndPoint) sendFinish(finish *msg.SendFinish) error {
  var buffer bytes.Buffer
  for i := 0; i < len(end.rcved); i++ {
    buffer.Write(rcved[i].Checksum)
  }
  if calc := CheckSum(buffer.Bytes()); BinaryCompare(calc, finish.Checksum) != 0 {
    return errors.New("snd finish check failed")
  }
  end.file.Close()
  return nil
}

func (end *EndPoint) recvInit(init *msg.RecvInit) error {
  end.name = *init.Name
  if info, err := os.Stat(end.name); err == nil || os.IsExist(err) {
    end.total = uint32(info.Size())
    end.block = BLOCK_SIZE
    end.cnt = (end.total + end.block - 1) / end.block
    if file, err := os.OpenFile(end.name, os.O_RDONLY, 0); err != nil {
      return err
    } else {
      end.file = file
    }
    return nil
  } else {
    return errors.New("file not exist")
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