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
  KDP_INPUT
  KDP_BREAK
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

type KDP struct {
  udp *net.UDPConn
  kcp *KCP
  buff []byte
  raddr *net.UDPAddr
  update bool
  close bool
  event chan *cmd
  arrived chan bool
}

func NewKDP(conv uint32, udp *net.UDPConn) *KDP {
  k := new(KDP)
  k.init(conv, udp)
  return k
}

func (k *KDP) init(conv uint32, udp *net.UDPConn) {
  k.udp = udp
  k.kcp = NewKCP(conv, udp)
  k.event = make(chan *cmd)
  k.arrived = make(chan bool)
  go k.demon()
}

func (k *KDP) Read(store []byte) (int, error) {
  if len(store) == 0 {
    return 0, errors.New("store size less than 1")
  }
  for !k.close {
    if len(k.buff) > 0 {
      cnt := copy(store, k.buff)
      if cnt < len(k.buff) {
        k.buff = k.buff[cnt:]
      } else {
        k.buff = nil
      }
      return cnt, nil
    }
    if data, err := k.read_once(); err != nil {
    } else {
      cnt := copy(store, data)
      if cnt < len(data) {
        k.buff = data[cnt:]
      }
      return cnt, nil
    }
    select {
    case <- time.After(1 * time.Second):
    case <- k.arrived:
    } 
  }
  return 0, errors.New("client closed")
}

func (k *KDP) read_once() ([]byte, error) {
  defer recover()
  if k.close {
    return nil, errors.New("kdp closed")
  }
  flow := make(chan *reply)
  defer close(flow)
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

func (k *KDP) Write(data []byte) error {
  defer recover()
  if data == nil || len(data) == 0 {
    return nil
  } else if k.close {
    return errors.New("kdp closed")
  }
  
  flow := make(chan *reply)
  defer close(flow)
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

func (k *KDP) Close() {
  defer recover()
  if k.close {
    return
  }
  k.close = true
  action := new(cmd)
  action.cmd = KDP_CLOSE
  k.event <- action
}

func (k *KDP) input(data []byte) error {
  defer recover()
  if data == nil || len(data) == 0 {
    return nil
  } else if k.close {
    return errors.New("kdp closed")
  }
  flow := make(chan *reply)
  defer close(flow)
  action := new(cmd)
  action.cmd = KDP_INPUT
  action.pipe = flow
  action.args = []interface{}{data}
  k.event <- action
  rslt := <- flow
  if rslt.err != nil {
    return rslt.err
  } else {
    select {
    case k.arrived <- true:
    default:
    }
    return nil
  }
}

func (k *KDP) execute(action *cmd) {
  go recover()
  switch action.cmd {
  case KDP_READ:
    k.execute_read(action)
  case KDP_WRITE:
    k.execute_write(action)
  case KDP_INPUT:
    k.execute_input(action)
  case KDP_CLOSE:
    k.execute_close(action)
  default:
    k.unknown_action(action)
  }
}

func (k *KDP) execute_close(action *cmd) {
  close(k.event)
  close(k.arrived)
}

func (k *KDP) execute_read(action *cmd) {
  rslt := new(reply)
  if data, err := k.kcp.receive(false); err != nil {
    rslt.err = err
  } else {
    rslt.rslt = []interface{}{data}
  }
  k.update = true
  go snd_rslt(rslt, action.pipe)
}

// need to write data here
func (k *KDP) execute_write(action *cmd) {
  rslt := new(reply)
  if action.args == nil || len(action.args) < 1 {
    rslt.err = errors.New("bad args")
  } else if data, ok := action.args[0].([]byte); !ok {
    rslt.err = errors.New("bad args")
  } else {
    rslt.err = k.kcp.send(data)
  }
  k.update = true
  go snd_rslt(rslt, action.pipe)
}

func (k *KDP) execute_input(action *cmd) {
  rslt := new(reply)
  if action.args == nil || len(action.args) < 1 {
    rslt.err = errors.New("bad args")
  } else if data, ok := action.args[0].([]byte); !ok {
    rslt.err = errors.New("bad args")
  } else {
    rslt.err = k.kcp.input(data)
  }
  k.update = true
  go snd_rslt(rslt, action.pipe)
}

// just report error
func (k *KDP) unknown_action(action *cmd) {
  rslt := new(reply)
  rslt.err = errors.New("unknown command")
  go snd_rslt(rslt, action.pipe)
}

func (k *KDP) snd_rslt(rslt *reply, pipe chan *reply) {
  defer recover()
  if pipe == nil {
    return
  }
  select {
    case <- time.After(1 * time.Millisecond):
    case pipe <- rslt:
  }
}


func (k *KDP) demon() {
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
  udp  *net.UDPConn
  pipe *KDP
  buff []byte
  close bool
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
    client.pipe = NewKDP(1, conn)
  }
  go client.demon()
  return client, nil
}

// Waiting data from server and 
func (client *Client) demon() {
  buffer := make([]byte, 4096)
  for !client.close {
    client.udp.SetReadDeadline(time.Now().Add(1 * time.Second))
    cnt, err := client.udp.Read(buffer)
    if cnt == 0 || err != nil {
      continue
    }
    data := make([]byte, cnt)
    copy(data, buffer)
    client.pipe.input(data)
  }
}

func (client *Client) Read(store []byte) (int, error) {
  if client.close {
    return 0, errors.New("client closed")
  }
  return client.pipe.Read(store)
}

func (client *Client) Write(data []byte) error {
  if client.close {
    return errors.New("client closed")
  }
  return client.pipe.Write(data)
}

func (client *Client) Close() {
  client.close = true
  client.pipe.Close()
  client.udp.Close()
}

type Server struct {
  udp   *net.UDPConn
  pipes map[string]*KDP

  event  chan *cmd
  accept chan *KDP
  close  bool
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
  go server.demon()
  return server, nil
}

func (server *Server) Accept() (*KDP, error) {
  for !server.close {
    select {
    case kdp := <- server.accept:
      return kdp, nil
    case <- time.After(time.Second):
    }
  }
  return nil, errors.New("server closed")
}

func (server *Server) Break(kdp *KDP) {
  defer recover()
  action := new(cmd)
  action.cmd = KDP_BREAK
  action.pipe = make(chan *reply)
  defer close(action.pipe)
  action.args = []interface{}{kdp}
  server.event <- action
  <- action.pipe
}

// Waiting data from client
func (server *Server) demon() {
  buffer := make([]byte, 2048)
  for {
    server.udp.SetReadDeadline(time.Now().Add(time.Second))
    cnt, saddr, err := server.udp.ReadFromUDP(buffer)
    if cnt == 0 || err != nil {
      continue
    }
    data := make([]byte, cnt)
    copy(data, buffer)
    if pipe, ok := server.pipes[saddr.String()]; !ok {
      pipe = NewKDP(1, server.udp)
      server.pipes[saddr.String()] = pipe
      pipe.input(data)
    } else {
      pipe.input(data)
    }
    select {
    case action := <- server.event:
      server.execute(action)
      if action.cmd == KDP_CLOSE {
        return
      }
    default:
    }
  }
}

func (server *Server) execute(action *cmd) {
  go recover()
  switch action.cmd {
  case KDP_CLOSE:
    server.exec_close(action)
  case KDP_BREAK:
    server.exec_break(action)
  default:
    server.unknown_action(action)
  }
}

func (server *Server) exec_close(action *cmd) {
  close(server.event)
  close(server.accept)
}

func (server *Server) exec_break(action *cmd) {
  rslt := new(reply)
  if len(action.args) <= 0 {
    rslt.err = errors.New("bad args")
  } else if pipe, ok := action.args[0].(*KDP); !ok {
    rslt.err = errors.New("bad args")
  } else {
    raddr := pipe.raddr.String()
    delete(server.pipes, raddr)
  }
  go snd_rslt(rslt, action.pipe)
}

func (server *Server) unknown_action(action *cmd) {
  rslt := new(reply)
  rslt.err = errors.New("unknown server action")
  action.pipe <- rslt
}

func snd_rslt(rslt *reply, pipe chan *reply) {
  defer recover()
  select {
  case pipe <- rslt:
  case <- time.After(time.Second):
  }
}
