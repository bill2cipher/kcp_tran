package transfer

import (
  "github.com/jellbean4/kcp_tran/kcp"
  "github.com/jellbean4/kcp_tran/msg"
)

type Sender struct {
  id     uint32 `desc:"identifier of this session"`
  total  uint32 `desc:"total size of the tranfered data"`
  block  uint32 `desc:"block size of the split data"`
  cnt    uint32 `desc:"how many blocks will be transfered"`
  queue  []*Block `desc:"where read but not sent block is stored"`
  send   []*Block `desc:"where sent but not verified block is stored"`
  recvd  []*Block `desc:"where recvd and verified block is stored"`
  socket *kcp.KcpSocket `desc:"kcp socket used to recv/send data"`
  name   string `desc:"file being read"`
  file   *os.File `desc:"file struct used to read data"`
}
