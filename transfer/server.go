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
    log.Printf("server read message failed %s", err.Error())
    return
  } 

  if mesg.SendInit != nil {
    err := s.proc_send(mesg.SendInit, sock)
    log.Printf("server proc send rslt %v", err)
  } else if mesg.RecvInit != nil {
    err := s.proc_recv(mesg.RecvInit, sock)
    log.Printf("server proc recv rslt %v", err)
  }
}

func (s *server) proc_recv(init *msg.RecvInit, sock Pipe) error {
  if info, err := os.Stat(*init.Name); err != nil {
    log.Printf("server check file %s info error %s", *init.Name, err.Error())
    return err
  } else if file, err := os.OpenFile(*init.Name, os.O_RDONLY, 0); err != nil {
    log.Printf("server open recved file %s error %s", *init.Name, err.Error())
    return err
  } else {
    point := NewEndPoint(1, sock)
    defer file.Close()
    return SendFileProc(point, info, file)
  }
}

func (s *server) proc_send(init *msg.SendInit, sock Pipe) error {
  var (
    cfile, file *os.File
    err  error
    fill bool
    point *EndPoint
  )
  flags := os.O_RDWR | os.O_CREATE
  name := s.dest + path.Base(*init.Name)
  name, fill = ChooseName(name)
  conf := ConfigName(name)
  
  defer CloseFiles(cfile, file)
  if cfile, err = os.OpenFile(conf, flags, 0600); err != nil {
    log.Printf("server open config file error %s", err.Error())
    return err
  } else if file, err = os.OpenFile(name, flags, 0600); err != nil {
    log.Printf("server open data file error %s", err.Error())
    return err
  }
  
  point = NewEndPoint(1, sock)
  if fill, err = LoadConfig(cfile, point); err != nil {
    log.Printf("server load config error %s", err.Error())
    return err
  }
  
  if fill, err = ParseInitMesg(init, point); err != nil {
    log.Printf("server parse init message failed %s", err.Error())
    return err
  } else if err = WriteConfig(cfile, point); err != nil {
    log.Printf("server write config failed %s", err.Error())
    return err
  }
  return RecvFileProc(point, file, cfile, fill)
}

