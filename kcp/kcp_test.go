package kcp

import (
  "net"
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
    udp, err = net.ListenUDP("udp4", local)
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
  
  skcp := NewKCP(server.Write)
  ckcp := NewKCP(client.Write)
}

