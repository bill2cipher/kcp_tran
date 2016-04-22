package transfer

import (
  "os"
  "log"
  "fmt"
  "path"
  "time"
  "errors"
  "io/ioutil"
  "encoding/binary"
)

import (
  "github.com/jellybean4/kcp_tran/kcp"
)

const (
  TIMEOUT    = 10 * time.Second
  BLOCK_SIZE = 1024 * 16
  ASK_WND = 5
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
  if err := SendFileProc(point, info, file); err != nil {
    return err
  }
  client.Sync()
  return nil
}

func SendFileProc(point *EndPoint, info os.FileInfo, file *os.File) error {
  if err := point.SendInit(info, 10 * time.Second); err != nil {
    return err
  }
  buffer := make([]byte, point.block)
  for true {
    var ask_idx []uint32
    if mesg, err := point.ReadMessageTimeout(TIMEOUT); err != nil {
      return err
    } else if mesg.AskPartial == nil {
      return errors.New("recv unknown mesg")
    } else if len(mesg.AskPartial.Index) == 0{
      return nil
    } else {
      ask_idx = append(ask_idx, mesg.AskPartial.Index...)
    }
    for _, idx := range ask_idx {
      pos := idx * point.block
      if cnt, err := ReadFile(file, int(pos), buffer); err != nil {
        return err
      } else if idx < point.cnt - 1 && uint32(cnt) < point.block {
        err_msg := fmt.Sprintf("read file content too small %d/%d %d/%d", idx, point.cnt, cnt, point.block)
        return errors.New(err_msg)
      } else if err := point.SendPartial(idx, buffer[:cnt]); err != nil {
        return err
      }
    }
  }
  return nil
}

func RecvFile(name string, dest string, raddr string) error {
  flags := os.O_WRONLY | os.O_CREATE
  var (
    cfile  *os.File
    file   *os.File
    client *kcp.Client
    point  *EndPoint
    err    error
    data   []byte
    fill   bool
  )
  defer CloseFiles(cfile, file)
  
  name, fill = ChooseName(dest + path.Base(name))
  if cfile, err = OpenConfigName(name); err != nil {
    return err
  } else if data, err = ReadConfig(cfile); err != nil {
    return err
  } else if file, err = os.OpenFile(dest, flags, 0660); err != nil {
    return err
  } else if client, err = kcp.Dial(raddr, 1); err != nil {
    return err
  } else if len(data) > 0 {    
    point = NewEndPoint(1, client)
    if err = point.Decode(data); err != nil {
      return err
    }
    if point.total == 0 || point.block == 0 || point.cnt == 0 {
      point.total, point.block, point.cnt = 0, 0, 0
      fill = true
    }
  } else {
    point = NewEndPoint(1, client)
  }

  if err = point.RecvInit(name, point.total, point.block, point.cnt); err != nil {
    return err
  } else if mesg, err := point.ReadMessageTimeout(TIMEOUT); err != nil {
    return err
  } else if mesg.SendInit == nil {
    return errors.New("do not recv send init mesg")
  } else if point.total != 0 && point.total != *mesg.SendInit.Total {
    return errors.New("file total size not match")
  } else if point.cnt != 0 && point.cnt != *mesg.SendInit.Cnt {
    return errors.New("file block count not match")
  } else if point.block != 0 && point.block != *mesg.SendInit.Block {
    return errors.New("fill block size not match")
  } else if point.total == 0 {
    fill = true
    point.SetInfo(*mesg.SendInit.Total, *mesg.SendInit.Block, *mesg.SendInit.Block)
  }
  return RecvFileProc(point, file, cfile, fill)
}

func RecvFileProc(point *EndPoint, file, cfile *os.File, fill bool) error {
  signal, first := make(chan error), true
  var (
    index uint32
    idxes []uint32
    count int
  )
  
  if fill {
    FillFile(file, point.total)
  }
  
  for len(point.ask_idx) > 0 || count > 0 {
    if first {
      idxes = point.AskIndex(5)
    } else {
      idxes = point.AskIndex(1)
    }
    count += len(idxes)
    if err := point.AskPartial(idxes); err != nil {
      return err
    } else if mesg, err := point.ReadMessageTimeout(TIMEOUT); err != nil {
      return err
    } else if mesg.SendPartial == nil {
      return errors.New("do not recv partial mesg")
    } else if first {
      partial := mesg.SendPartial
      pos, data := *partial.Index * point.block, partial.Data
      go FlushFile(file, pos, data, signal)
    } else if err = <- signal; err != nil {
      return err
    } else {
      partial := mesg.SendPartial
      pos, data := *partial.Index * point.block, partial.Data
      count--
      go FlushFile(file, pos, data, signal)
    }
    log.Printf("recv progress %d/%d", index, point.cnt)
    first = false
    index++
  }
  return nil
}

func FlushFile(file *os.File, pos uint32, data []byte, signal chan error) {
  log.Printf("writing offset %d", int64(pos))
  if cnt, err := file.WriteAt(data, int64(pos)); err != nil {
    signal <- err
  } else if cnt < len(data) {
    signal <- errors.New("write data too small")
  }
  signal <- nil
}

func OpenConfigName(name string) (*os.File, error) {
  flags := os.O_APPEND | os.O_WRONLY | os.O_CREATE
  if file, err := os.OpenFile(name, flags, 0660); err != nil {
    return nil, err
  } else {
    return file, err
  }
}

func ReadConfig(file *os.File) ([]byte, error) {
  if data, err := ioutil.ReadAll(file); err != nil {
    return nil, err
  } else {
    return data, nil
  }
}

func AppendConfig(file *os.File, index uint32) {
  store := make([]byte, 4)
  binary.LittleEndian.PutUint32(store, index)
  file.Write(store)
}

func ChooseName(name string) (string, bool) {
  cnt := 1
  base := name
  for true {
    if _, err := os.Stat(name + ".download"); err == nil {
      return name, false
    } else if _, err := os.Stat(name); err != nil {
      return name, true
    }
    name = fmt.Sprintf("%s.%d", base, cnt)
    cnt++
  }
  return name, true
}

