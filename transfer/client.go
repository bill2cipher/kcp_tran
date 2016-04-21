package transfer

import (
  "os"
  "io"
  "log"
  "errors"
)

import (
  "github.com/jellybean4/kcp_tran/kcp"
)

const (
  BLOCK_SIZE = 1024 * 1024 * 4
)

func SendFile(name string, raddr string) error {
  var (
    info os.FileInfo
    file *os.File
    client *kcp.Client
    err error
  )
  if info, err = os.Stat(name); err != nil {
    return err
  } else if file, err = os.OpenFile(name, os.O_RDONLY, 0); err != nil {
    return err
  } else if client, err = kcp.Dial(raddr, 1); err != nil {
    file.Close()
    return err
  }
  point := NewEndPoint(1, client)
  defer file.Close()
  defer client.Close()
  defer point.Close()
  return SendFileProc(point, &info, file)
}

func SendFileProc(point *EndPoint, info *os.FileInfo, file *os.File) error {
  buffer := make([]byte, BLOCK_SIZE)
  if err := point.SendInit(info); err != nil {
    return err
  }
  for true {
    if cnt, err := file.Read(buffer); err != nil && err != io.EOF {
      return err
    } else if err == io.EOF || cnt == 0 {
      break
    } else if err := point.SendPartial(buffer[:cnt]); err != nil {
      return err
    }
  }
  if err := point.SendFinish(); err != nil {
    return err
  }
  log.Printf("file %s send finish", name)
  return nil
}

func RecvFile(name string, dest string, raddr string) error {
  flags := os.O_APPEND | os.O_WRONLY | os.O_CREATE
  var (
    file   *os.File
    client *kcp.Client
    err    error
    data   []byte
    pos    uint32
  )
  defer func() {
    if err != nil {
      os.Remove(dest)
    }
  }()
  
  if file, err = os.OpenFile(dest, flags, 0660); err != nil {
    return err
  } else if client, err = kcp.Dial(raddr, 1); err != nil {
    file.Close()
    return err
  }
  defer file.Close()
  point := NewEndPoint(1, client)
  if err := point.RecvInit(name); err != nil {
    return err
  }
  return RecvFileProc(point, file)
}

func RecvFileProc(point *EndPoint, file *os.File) error {
  signal, first := make(chan error), true
  for !point.Finished() {
    if data, pos, err = point.ReadPartial(); err != nil {
      return err
    } else if first {
      go FlushFile(file, pos, data, signal)
    } else if err = <- signal; err != nil {
      return err
    } else {
      go FlushFile(file, pos, data, signal)
    }
    first = false
  }
  return nil
}

func FlushFile(file *os.File, pos uint32, data []byte, signal chan error) {
  if _, err := file.Seek(int64(pos), os.SEEK_CUR); err != nil {
    signal <- err
  } else if cnt, err := file.Write(data); err != nil {
    signal <- err
  } else if cnt < len(data) {
    signal <- errors.New("write data too small")
  }
  signal <- nil
}