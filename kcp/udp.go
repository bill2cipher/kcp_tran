package kcp

import (
  "net"
  "time"
  "errors"
)

const (
  SERVER_ADDR = "0.0.0.0:10878"
  KDP_INTERVAL = 10 * time.Millisecond
)

const (
  KDP_CLOSE = iota
  KDP_READ
  KDP_WRITE
)

func clock() uint32 {
  Now := time.Now().UnixNano()
  return uint32(Now / 1000000) & 0xFFFFFFFF
}

type cmd struct {
  cmd  uint32
  pipe chan *reply
  args []interface{}
}

type reply struct {
  err  error
  rslt []interface{}
}

type kdp struct {
  udp *net.UDPConn
  kcp *KCP
  
  update bool
  close bool
  event chan *cmd
}

func (k *kdp) init(conv uint32, udp *net.UDPConn) {
  k.udp = udp
  k.kcp = NewKCP(conv, udp)
  k.event = make(chan *cmd, 10)
  go k.demon()
}

func (k *kdp) read() ([]byte, error) {
  flow := make(chan *reply)
  action := new(cmd)
  action.cmd = KDP_READ
  action.pipe = flow
  
  k.event <- action
  rslt := <- flow
  if rslt.err != nil {
    return nil, rslt.err
  } else {
    data := rslt.rslt[0].([]byte)
    return data, nil
  }
}

func (k *kdp) write(data []byte) error  {
  if data == nil || len(data) == 0 {
    return nil
  }
  
  flow := make(chan *reply)
  action := new(cmd)
  action.cmd = KDP_WRITE
  action.pipe = flow
  action.args = []interface{}{data}
  k.event <- action
  rslt := <- flow
  if rslt.err != nil {
    return rslt.err
  } else {
    return nil
  }
}

func (k *kdp) execute(action *cmd) {
  switch action.cmd {
  case KDP_CLOSE:
    k.execute_close(action)
  case KDP_READ:
    k.execute_read(action)
  case KDP_WRITE:
    k.execute_write(action)
  default:
    k.unknown_action(action)
  }
}

func (k *kdp) execute_read(action *cmd) {
  rslt := new(reply)
  if data, err := k.kcp.receive(false); err != nil {
    rslt.err = err
  } else {
    rslt.rslt = []interface{}{data}
  }
  
}

// need to write data here
func (k *kdp) execute_write(action *cmd) {
  rslt := new(reply)
  if action.args == nil || len(action.args) < 1 {
    rslt.err = errors.New("bad args")
  } else if data, ok := action.args[0].([]byte); !ok {
    rslt.err = errors.New("bad args")
  } else {
    rslt.err = k.kcp.send(data)
  }
  action.pipe <- rslt
}

// need to do nothing here
func (k *kdp) execute_close(action *cmd) {
}

// just report error
func (k *kdp) unknown_action(action *cmd) {
  rslt := new(reply)
  rslt.err = errors.New("unknown command")
  go func() {action.pipe <- rslt}()
}

func (k *kdp) demon() {
  trigger := time.NewTicker(KDP_INTERVAL)
  var update_time uint32
  for {
    select {
    case <- trigger.C:
      current := clock()
      if k.update || update_time <= current {
        k.kcp.update(current)
        update_time = k.kcp.check(current)
      }
    case action := <- k.event:
      k.execute(action)
      if k.close {
        return
      }
    }
  }
}


type Client struct {
  udp *net.UDPConn
}

func Dial(raddr string) (*Client, error) {
  client := new(Client)
  if remote, err := net.ResolveUDPAddr("udp4", raddr); err != nil {
    return nil, err
  } else if conn, err := net.DialUDP("udp4", nil, remote); err != nil {
    defer conn.Close()
    return nil, err
  } else {
    client.udp = conn
  }
  go client.play()
  return client, nil
}

// Waiting data from server and 
func (client *Client) play() {
  
}

type Server struct {
  udp *net.UDPConn
}

func Listen(laddr string) (*Server, error) {
  server := new(Server)
  if local, err := net.ResolveUDPAddr("udp4", SERVER_ADDR); err != nil {
    return nil, err
  } else if conn, err := net.ListenUDP("udp4", local); err != nil {
    defer conn.Close()
    return nil, err
  } else {
    server.udp = conn
  }
  go server.play()
  return server, nil
}

// Waiting data from client
func (server *Server) play() {
  
}