package transfer

import (
  "testing"
	"os"
)

func TestFill(t *testing.T) {
  file, err := os.OpenFile("/tmp/d", os.O_TRUNC | os.O_WRONLY | os.O_CREATE, 0660)
  if err != nil {
    t.Errorf("open file failed %s", err.Error())
  }
  if err := FillFile(file, 12173912); err != nil {
    t.Errorf("fill file failed %s", err.Error())
  }
  file.Close()
}


func TestEndPoint(t *testing.T) {
  go Serve()
  
}