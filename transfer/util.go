package transfer

import (
  "os"
	"io"
  "crypto/md5"
  "encoding/binary"
)

import "github.com/golang/protobuf/proto"
import "github.com/jellybean4/kcp_tran/msg"

func CheckSum(data []byte) []byte {
  rslt := md5.Sum(data)
  return rslt[:]
}

func BinaryCompare(a, b []byte) int {
  var mlen int
  if len(a) > len(b) {
    mlen = len(b)
  } else {
    mlen = len(a)
  }
  for i := 0; i < mlen; i++ {
    if a[i] > b[i] {
      return 1
    } else if a[i] < b[i] {
      return -1
    }
  }
  
  if len(a) > len(b) {
    return 1
  } else if len(a) < len(b) {
    return -1
  } else {
    return 0
  }
}

func FillFile(file *os.File, total uint32) error {
  cnt := total / 512
  left := total - 512 * cnt
  buffer := make([]byte, 512)
  for i := uint32(0); i < cnt; i++ {
    if _, err := file.Write(buffer); err != nil {
      return err
    }
  }
  if left != 0 {
    if _, err := file.Write(buffer[:left]); err != nil {
      return err
    }
  }
  return nil
}

func ReadFile(file *os.File, pos int, buffer []byte) (int, error) {
  var size int
  for size < len(buffer) {
    if cnt, err := file.ReadAt(buffer[size:], int64(pos + size)); err != nil && err != io.EOF {
      return 0, err
    } else if err == io.EOF {
      return cnt + size, nil
    } else {
      size += cnt
    }
  }
  return size, nil
}


func ReadHeader(store []byte, sock Pipe) (uint32, error) {
  if err := ReadData(store, sock); err != nil {
    return 0, err
  } else {
    dlen := binary.LittleEndian.Uint32(store)
    return dlen, nil
  }
}

func ReadMessage(dlen uint32, sock Pipe) ([]byte, error) {
  buffer := make([]byte, dlen)
  if err := ReadData(buffer, sock); err != nil {
    return nil, nil
  } else {
    return buffer, nil
  }
}

func ReadData(store []byte, sock Pipe) error {
  size := 0
  for size < len(store) {
    if cnt, err := sock.Read(store[size:]); err != nil {
      return nil
    } else {
      size += cnt
    }
  }
  return nil
}

func EncodeMesg(mesg *msg.Transfer) ([]byte, error) {
  header := make([]byte, 4)
  if body, err := proto.Marshal(mesg); err != nil {
    return nil, err
  } else {
    dlen := uint32(len(body))
    binary.LittleEndian.PutUint32(header, dlen)
    return append(header, body...), nil
  }
}

func CloseFiles(files... *os.File) {
  for _, f := range files {
    if f == nil {
      continue
    }
    f.Close()
  }
}