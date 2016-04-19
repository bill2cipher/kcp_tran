package transfer

import (
  "os"
  "log"
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

const (
  BLOCK_SIZE = 1024 * 1024 * 4
)

type EndPoint struct {
  id      uint32 `desc:"identifier of this EndPoint"`
  total   uint32 `desc:"total size of the tranfered data"`
  block   uint32 `desc:"block size of the split data"`
  cnt     uint32 `desc:"how many blocks will be transfered"`
  rcv_buf []*msg.SendPartial `desc:"where recvd but not verified block is stored"`
  recvd   []*msg.SendPartial `desc:"where recvd and verified block is stored"`
  snd_buf []*msg.SendPartial `desc:"where data need to be sent"`
  sent    []*msg.SendPartial `desc:"where data sent but not ack"`
  sock    *kcp.KDP `desc:"kcp socket used to recv/send data"`
  name    string `desc:"file being written"`
  file    *os.File `desc:"file struct used to write/read data"`
  close   bool `desc:"if this end point is closed"`
}

func NewEndPoint(id uint32, sock *kcp.KDP) *EndPoint {
  end := new(EndPoint)
  return end
}

func (end *EndPoint) init(id uint32, sock *kcp.KDP) {
  end.id = id
  end.sock = sock
}

func (end *EndPoint) demon() {
  store := make([]byte, 4)
  trans := new(msg.Transfer)
  for !end.close {
    if dlen, err := end.readLen(store); err != nil {
      log.Printf("read msg len failed %s", err.Error())
      continue
    } else if data, err := end.readMsg(dlen); err != nil {
      log.Printf("read msg failed %d %s", dlen, err.Error())
      continue
    } else if err := proto.Unmarshal(data, trans); err != nil {
      log.Printf("decode proto msg failed %s", err.Error())
      continue
    } else {
      end.execute(trans)
    }
  }
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
  case trans.RecvPartial != nil:
    return end.recvPartial(trans.RecvPartial)
  case trans.RecvStatus != nil:
    return end.recvStatus(trans.RecvStatus)
  case trans.RecvFinish != nil:
    return end.recvFinish(trans.RecvFinish)
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
  } else {
    return nil
  }
}

func (end *EndPoint) sendFinish(finish *msg.SendFinish) error {
  for i := 0; i < len(end.rcv_buf); i++ {
    end.rcv_buf[i]
  }
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

func (end *EndPoint) recvPartial(partial *msg.RecvPartial) error {
  
}

func (end *EndPoint) recvStatus(status []*msg.SendPartial) error {
  
}

func (end *EndPoint) recvFinish(finish *msg.RecvFinish) error {
  
}



func (end *EndPoint) readLen(store []byte) (uint32, error) {
  if err := end.readData(store); err != nil {
    return 0, err
  } else {
    dlen := binary.LittleEndian.Uint32(store)
    return dlen, nil
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