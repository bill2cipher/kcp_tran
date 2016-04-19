package kcp

import (
  "log"
  "fmt"
  "strings"
  "testing"
)

var finish = make(chan bool)

func TestUDP(t *testing.T) {
  server, err := Listen("0.0.0.0:9010", 1)
  if err != nil {
    log.Printf("create server failed %v", err)
  }
  
  go serve(server, t, 1)
  snd(0, t)
  <- finish
  server.Close()
}

func TestMulti(t *testing.T) {
  server, err := Listen("0.0.0.0:9010", 1)
  if err != nil {
    log.Printf("create server failed %v", err)
  }
  go snd(0, t)
  go snd(3000, t)
  go snd(6000, t)
  serve(server, t, 3)
  for i := 0; i < 3; i++ {
    <- finish
  }
  server.Close()
}


func snd(start int, t *testing.T) {
  client, err := Dial("127.0.0.1:9010", 1)
  if err != nil {
    log.Printf("dial server failed")
  }
  for i := start; i < start + 3000; i++ {
    msg := strings.Repeat(fmt.Sprintf("msg%d", i), 700)
    if err := client.Write([]byte(msg)); err != nil {
      log.Printf("client write failed %v", err)
    }
  }
  //client.Close()
  log.Printf("%d snd finished", start)
}

func serve(s *Server, t *testing.T, cnt int) {
  for j := 0; j < cnt; j++ {
    go func(id int) {
      sock, err := s.Accept()
      if err != nil {
        log.Printf("accept sock failed %v", err)
        return
      }
      log.Printf("%d accepted", id)
      buffer := make([]byte, 5000)
      for i := 0; i < 3000; i++ {
        if _, err := sock.Read(buffer); err != nil {
          log.Printf("sock read failed %v", err)
        } else {
          log.Printf("rcv client %d progress %d/%d", id, i, 3000)
        }
      }
      finish <- true 
    }(j)
  }
}
