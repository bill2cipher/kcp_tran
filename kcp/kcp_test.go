package kcp

import (
  "net"
  "testing"
)

func before(remote, local string, rtype int, t *testing.T) *net.UDPConn {
  laddr, err := net.ResolveUDPAddr("udp4", local)
  if err != nil {
    t.Errorf("could resolve local ip %v", err)
    return nil
  }

  raddr, err := net.ResolveUDPAddr("udp4", remote)
  if err != nil {
    t.Errorf("could not resolve remote ip %v", err)
    return nil
  }
  var udp *net.UDPConn
  if rtype == 0 {
    udp, err = net.DialUDP("udp4", laddr, raddr)
  } else if rtype == 1 {
    udp, err = net.ListenUDP("udp4", laddr)
  }
  
  if err != nil {
    t.Errorf("could not build udp %v", err)
    return nil
  }
  return udp
}

func TestKcp(t *testing.T) {
  server := before("", "127.0.0.1:12345", 1, t)
  client := before("127.0.0.1:12345", "127.0.0.1:23456", 0, t)
  
  skcp := NewKCP(1, server)
  ckcp := NewKCP(1, client)
  client_proc(ckcp, t)
  server_proc(skcp, t)
}

func server_proc(server *KCP, t *testing.T) {
  sock := server.writer.(*net.UDPConn)
  data := make([]byte, 1024)
  for i := 0; i < 100; i++ {
    cnt, err := sock.Read(data)
    if err != nil {
      t.Errorf("read data error %v\n", err)
    }
    t.Logf("rcv data len %d\n", cnt)
    server.input(data[:cnt])
    _, err = server.receive(false)
    if err != nil {
      t.Errorf("receive data error %v\n", err)
    }
  }
}

func client_proc(client *KCP, t *testing.T) {
  for i := 0; i < 100; i++ {
    data := big()
    client.send(data)
    client.set_nodelay(0, 10, 0, 0)
    client.update(0)
  }
}

func big() []byte {
  rslt := make([]byte, 2048 * 3)
  for i := 0; i < len(rslt); i++ {
    rslt[i] = 'a'
  }
  return rslt
}


