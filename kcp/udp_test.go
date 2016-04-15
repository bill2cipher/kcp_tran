package kcp

import (
  "log"
  "testing"
)

func TestUDP(t *testing.T) {
  server, err := Listen("0.0.0.0:9090")
  if err != nil {
    t.Errorf("create server failed %v", err)
  }
  
  log.Printf("starting accept client...")
  go serve(server, t)
}


func snd() {
  client, err := Dial("127.0.0.1:9090")
  if err != nil {
    
  }
}

func serve(s *Server, t *testing.T) {
  for {
    s.Accept()
  }
}