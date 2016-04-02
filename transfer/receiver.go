package transfer

import (
  "os"
)

import (
  "github.com/jellybean4/kcp_tran/kcp"
  "github.com/jellybean4/kcp_tran/msg"
)

type Block struct {
  msg.SendPartial
  data []byte 
}

type Receiver struct {
  id     uint32 `desc:"identifier of this session"`
  total  uint32 `desc:"total size of the tranfered data"`
  block  uint32 `desc:"block size of the split data"`
  cnt    uint32 `desc:"how many blocks will be transfered"`
  queue  []*Block `desc:"where recvd but not verified block is stored"`
  recvd  []*Block `desc:"where recvd and verified block is stored"`
  socket *kcp.KcpSocket `desc:"kcp socket used to recv/send data"`
  name   string `desc:"file being written"`
  file   *os.File `desc:"file struct used to write data"`
}


func NewReceiver(socket *kcp.KcpSocket) (*Receiver, error) {
  recv := new(Receiver)
  if err := recv.init(socket); err != nil {
    return nil, err
  }
  return recv, nil
}

func (s *Receiver) init(socket *kcp.KcpSocket) error {
  s.socket = socket
  s.id = 0
  return nil 
}

func (s *Receiver) Router() error {
  return nil
}
