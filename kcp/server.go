package kcp

type KcpServer struct {
  
}

func NewKcpServer() *KcpServer {
  server := new(KcpServer)
  server.init()
  return server
}

func (kcp *KcpServer) init() error {
  return nil
}

func (kcp *KcpServer) Listen() *KcpClient {
  return nil
}

