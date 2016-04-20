package transfer

type Client struct {
  
}

func SendFile(name string) error {
  client, err := kcp.Dial("127.0.0.1:8765", 1)
  if err != nil {
  }
  return nil
}

func RecvFile(name string, host string) error {
  request, init := new(msg.Transfer), new(msg.RecvInitRp)
  
}