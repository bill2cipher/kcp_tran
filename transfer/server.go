package transfer

import (
  "github.com/jellybean4/kcp_tran/kcp"
)

func Serve() error {
   server, err := kcp.Listen("0.0.0.0:8765", 1)
   if err != nil {
     return err
   }
   for {
     kdp, err := server.Accept()
     if err != nil {
       return err
     }
     go process(point) 
   }
}


func process(sock Pipe) {
  rtype, msg := ReadMessage(sock)
  if rtype != 1 {
    log.Printf("message type error")
    return
  }
  if transfer, ok := msg.(*msg.Transfer); !ok {
    log.Printf("message convert type error")
  } else if transfer.SendInit != nil {
    err := proc_send(transfer.SendInit.Name, sock)
    log.Printf("proc send rslt %v", err)
  } else if transfer.RecvInit != nil {
    err := proc_recv(transfer.RecvInit.Name, sock)
    log.Printf("proc recv rslt %v", err)
  }
}

func proc_recv(name string, sock *kcp.KDP) error {
  if info, err := os.Stat(name); err != nil {
    return err
  } else if file, err := os.OpenFile(name, os.O_RDONLY, 0); err != nil {
    return err
  } else {
    point := NewEndPoint(1, sock)
    defer file.Close()
    defer point.Close()
    return SendFileProc(point, info, file)
  }
}

func proc_send(name string, sock *kcp.KDP) error {
  flags := os.O_APPEND | os.O_WRONLY | os.O_CREATE
  if file, err := os.OpenFile(name, flags, 0600); err != nil {
    return err
  }
  point := NewEndPoint(1, sock)
  point.ReplySendInit()
  return RecvFileProc(point, file)
}