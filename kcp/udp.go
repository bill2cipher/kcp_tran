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
  updated chan bool
  close bool
  event chan *cmd
  arrived chan bool
}

func NewKDP(conv uint32, udp *net.UDPConn, raddr *net.UDPAddr) *KDP {
  k := new(KDP)
  k.init(conv, udp, raddr)
  return k
}

func (k *KDP) init(conv uint32, udp *net.UDPConn, raddr *net.UDPAddr) {
  k.udp = udp
  k.raddr = raddr
  k.kcp = NewKCP(conv, k.output)
  k.event = make(chan *cmd)
  k.arrived = make(chan bool)
  k.updated = make(chan bool)
  k.kcp.set_nodelay(1, 10, 2, 1)
  go k.demon()
}

func (k *KDP) output(data []byte) (int, error) {
  return k.udp.WriteToUDP(data, k.raddr)
}

func (k *KDP) Sync() {
  for k.kcp.snd_buf.Len() + k.kcp.snd_queue.Len() > 0 {
  <-time.After(500 * time.Millisecond)
  }
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
  } 
  data := rslt.rslt[0].([]byte)
  return data, nil
}

func (k *KDP) Write(data []byte) error {
  defer recover()
  if data == nil || len(data) == 0 {
    return nil
  } else if k.close {
    return errors.New("kdp closed")
  }
  for true {
    select {
    case <-k.updated:
    case <-time.After(time.Second):
    }
    if k.kcp.snd_queue.Len() > 0 {
      continue
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
    }
    return nil
  }
  return nil
}

func (k *KDP) Close() {
  defer recover()
  if k.close {
    return
  }
  k.close = true
  action := new(cmd)
  action.cmd = KDP_CLOSE
  action.pipe = make(chan *reply)
  k.event <- action
  <- action.pipe
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
  defer recover()
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
  go snd_rslt(nil, action.pipe)
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
    case <- time.After(1 * time.Second):
    case pipe <- rslt:
  }
}

func (k *KDP) demon() {
  trigger := time.NewTicker(KDP_INTERVAL)
  var update_time uint32
  for !k.close {
    select {
    case <- trigger.C:
      current := clock()
      if k.update || update_time <= current {
        k.kcp.update(current)
        update_time = k.kcp.check(current)
        select {
        case k.updated <- true:
        default:
        }
      }
    case action := <- k.event:
      k.execute(action)
    }
  }
}

type Client struct {
  udp  *net.UDPConn
  pipe *KDP
  buff []byte
  close bool
}

func Dial(raddr string, id uint32) (*Client, error) {
  client := new(Client)
  if local, err := net.ResolveUDPAddr("udp4", "0.0.0.0:0"); err != nil {
    return nil, err
  } else if remote, err := net.ResolveUDPAddr("udp4", raddr); err != nil {
    return nil, err 
  } else if conn, err := net.ListenUDP("udp4", local); err != nil {
    defer conn.Close()
    return nil, err
  } else {
    client.udp = conn
    client.pipe = NewKDP(id, conn, remote)
  }
  go client.demon()
  return client, nil
}

// Waiting data from server and 
func (client *Client) demon() {
  buffer := make([]byte, 4096)
  for !client.close {
    client.udp.SetReadDeadline(time.Now().Add(10 * time.Second))
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

func (client *Client) Sync() {
  client.pipe.Sync()
}

type Server struct {
  udp   *net.UDPConn
  pipes map[string]*KDP
  id    uint32
  event  chan *cmd
  accept chan *KDP
  close  bool
}

func Listen(laddr string, id uint32) (*Server, error) {
  server := new(Server)
  server.init()
  if local, err := net.ResolveUDPAddr("udp4", laddr); err != nil {
    return nil, err
  } else if conn, err := net.ListenUDP("udp4", local); err != nil {
    defer conn.Close()
    return nil, err
  } else {
    server.udp = conn
    server.id = id
  }
  go server.demon()
  return server, nil
}

func (server *Server) init() {
  server.pipes = make(map[string]*KDP)
  server.event = make(chan *cmd)
  server.accept = make(chan *KDP, 1024)
  server.close = false
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

func (server *Server) Close() {
  action := new(cmd)
  action.cmd = KDP_CLOSE
  action.pipe = make(chan *reply)
  server.event <- action
  <- action.pipe
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
  for !server.close {
    server.udp.SetReadDeadline(time.Now().Add(time.Second))
    cnt, raddr, err := server.udp.ReadFromUDP(buffer)
    if cnt == 0 || err != nil {
    } else {
      data := make([]byte, cnt)
      copy(data, buffer)
      if pipe, ok := server.pipes[raddr.String()]; !ok {
        pipe = NewKDP(server.id, server.udp, raddr)
        server.pipes[raddr.String()] = pipe
        pipe.input(data)
        server.accept <- pipe
      } else {
        pipe.input(data)
      }
    }
    select {
    case action := <- server.event:
      server.execute(action)
    default:
    }
  }
}

func (server *Server) execute(action *cmd) {
  defer recover()
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
  for _, v := range server.pipes {
    v.Close()
  }
  server.close = true
  close(server.event)
  close(server.accept)
  server.udp.Close()
  go snd_rslt(nil, action.pipe)
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
