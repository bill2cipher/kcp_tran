package kcp

import (
  "testing"
)

func TestInsert(t *testing.T) {
  queue := NewQueue()
  for i := 0; i < 100; i++ {
    queue.Insert(i)
  }
  
  if queue.Len() != 100 {
    t.Errorf("queue len error %d", queue.Len())
  }
  
  entry := queue.prev
  for i := 0; i < 100; i++ {
    val := entry.val.(int)
    if val != i {
      t.Errorf("queue insert rslt error %d", val)
    }
    entry = entry.prev
  }
  
  for cnt, entry := 0, queue.next; entry != queue; cnt++ {
    next := entry.next
    if cnt == 4 || cnt == 20 || cnt == 55 {
      queue.Delete(entry)
    }
    entry = next
  }
  if queue.Len() != 100 - 3 {
    t.Errorf("queue len not match after delete")
  }
  
  cnt := 0
  for entry := queue.next; entry != queue; entry = entry.next {
    cnt++
  }
  if cnt != 100 - 3 {
    t.Errorf("queue len not match after delete2")
  }
  
  for cnt, entry := 0, queue; cnt < 100 && entry != queue; cnt++ {
    val := entry.val.(int)
    if cnt == 4 || cnt == 20 || cnt == 55 {
      if val == cnt {
        t.Errorf("queue entry not deleted")
      }
      continue
    }
    if cnt != val {
      t.Errorf("queue val not match")
    }
    entry = entry.next
  }
}

func TestPush(t *testing.T) {
  queue := NewQueue()
  if queue.Len() != 0 {
    t.Errorf("empty len not zero")
  }
  for i := 0; i < 100; i++ {
    queue.Push(i)
  }
  
  if queue.Len() != 100 {
    t.Errorf("full queue not 100")
  }
  
  entry := queue.next
  for i := 0; i < 100; i++ {
    val := entry.val.(int)
    if val != i {
      t.Errorf("queue push rslt error %d", val)
    }
    entry = entry.next
  }
  
  cnt := 0
  for queue.Len() != 0 {
    entry := queue.Pop()
    val := entry.val.(int)
    if val != cnt {
      t.Errorf("queue pop error")
    }
    cnt++
  }
}
