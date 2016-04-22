package transfer

import (
  "os"
  "log"
  "path"
)

import (
  "github.com/jellybean4/kcp_tran/kcp"
	"github.com/jellybean4/kcp_tran/msg"
)

type server struct {
  host, dest string
}

func Serve(host string, dest string) error {
  s := new(server)
  s.host, s.dest = host, dest

  server, err := kcp.Listen(host, 1)
  if err != nil {
    return err
  }
  for {
    kdp, err := server.Accept()
    if err != nil {
      return err
    }
    go s.process(kdp) 
  }
}

func (s *server) process(sock Pipe) {
  point := NewEndPoint(1, sock)
  mesg, err := point.ReadMessageTimeout(TIMEOUT)
  if err != nil {
    log.Printf("read message failed %s", err.Error())
    return
  } 

  if mesg.SendInit != nil {
    err := s.proc_send(mesg.SendInit, sock)
    log.Printf("proc send rslt %v", err)
  } else if mesg.RecvInit != nil {
    err := s.proc_recv(mesg.RecvInit, sock)
    log.Printf("proc recv rslt %v", err)
  }
}

func (s *server) proc_recv(init *msg.RecvInit, sock Pipe) error {
  if info, err := os.Stat(*init.Name); err != nil {
    return err
  } else if file, err := os.OpenFile(*init.Name, os.O_RDONLY, 0); err != nil {
    return err
  } else {
    point := NewEndPoint(1, sock)
    defer file.Close()
    return SendFileProc(point, info, file)
  }
}

func (s *server) proc_send(init *msg.SendInit, sock Pipe) error {
  flags := os.O_WRONLY | os.O_CREATE
  name := s.dest + path.Base(*init.Name)
  conf := name + ".download"
  var cfile, file *os.File
  var err error
  defer func() {
    if cfile != nil {
      cfile.Close()
    }
    if file != nil {
      file.Close()
    }
  }()
  if cfile, err = os.OpenFile(conf, flags, 0600); err != nil {
    return err
  } else if file, err = os.OpenFile(name, flags, 0600); err != nil {
    return err
  } else {
    point := NewEndPoint(1, sock)
    point.SetInfo(*init.Total, *init.Block, *init.Cnt)
    return RecvFileProc(point, file, cfile, true)
  }
}

