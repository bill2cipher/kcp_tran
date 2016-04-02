package kcp

type KcpClient struct {
  
}

func NewKcpClient() *KcpClient {
  client := new(KcpClient)
  client.init()
  return client
}

func (kcp *KcpClient) init() error {
  return nil
}

func (kcp *KcpClient) Recv() ([]byte, error) {
  return nil, nil
}

func (kcp *KcpClient) Send(data []byte) error {
  return nil
}
