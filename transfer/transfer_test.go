package transfer

import "testing"
import "log"

func TestTran(t *testing.T) {
  go func() {
    if err := Serve("0.0.0.0:8765", "/tmp/testdir/"); err != nil {
      log.Printf("listen serve failed %s", err.Error())
    }
  }()
  if err := SendFile("/tmp/tel-account.tar.gz", "127.0.0.1:8765"); err != nil {
    log.Printf("send file rslt %s", err.Error()) 
  }
}
