package kcp

type Queue struct {
  prev *Queue
  next *Queue
  val  interface{}
}

func NewQueue() *Queue {
  queue := new(Queue)
  queue.init()
  return queue
}

func (q *Queue) init() {
  cnt := uint32(0)
  q.prev = q
  q.next = q
  q.val = &cnt
}

func (q *Queue) Len() uint32 {
  return *q.val.(*uint32)
}

func (q *Queue) Before(pos *Queue, val interface{}) {
  entry := new(Queue)
  entry.val = val
  q.BeforeNode(pos, entry)
}

func (q *Queue) BeforeNode(pos, node *Queue) {
  prev := pos.prev
  prev.next, pos.prev = node, node
  node.prev, node.next = prev, pos
  *q.val.(*uint32)++
}

func (q *Queue) After(pos *Queue, val interface{}) {
  entry := new(Queue)
  entry.val = val
  q.AfterNode(pos, entry)
}

func (q *Queue) AfterNode(pos, node *Queue) {
  next := pos.next
  pos.next, next.prev = node, node
  node.prev, node.next = pos, next
  *q.val.(*uint32)++
}

func (q *Queue) PushNode(node *Queue) {
  node.prev = q.prev
  node.next = q
  
  q.prev.next = node
  q.prev = node
  *q.val.(*uint32)++
}

func (q *Queue) Push(val interface{}) {
  entry := new(Queue)
  entry.val = val
  q.PushNode(entry)
}

func (q *Queue) InsertNode(entry *Queue) {
  entry.prev, entry.next = q, q.next
  
  q.next, q.next.prev = entry, entry
  *q.val.(*uint32)++
}

func (q *Queue) Insert(val interface{}) {
  entry := new(Queue)
  entry.val = val
  q.InsertNode(entry)
}

func (q *Queue) Pop() *Queue {
  if q.Len() == 0 {
    return nil
  }
  
  entry := q.next
  q.next = entry.next
  entry.next.prev = q
  *q.val.(*uint32)--

  entry.next = nil
  entry.prev = nil
  return entry
}

func (q *Queue) PopVal() interface{} {
  if entry := q.Pop(); entry == nil {
    return nil
  } else {
    return entry.val
  }
}

func (q *Queue) Delete(entry *Queue) {
  if q == entry {
    return
  }
  
  entry.next.prev, entry.prev.next = entry.prev, entry.next
  *q.val.(*uint32)--
}


type Iterator struct {
  head *Queue
  cur  *Queue
}

func NewIterator(q *Queue) *Iterator {
  iter := new(Iterator)
  iter.head = q
  iter.cur = iter.head
  return iter
} 

func (iter *Iterator) Valid() bool {
  return iter.cur != iter.head
}

func (iter *Iterator) Next() {
  iter.cur = iter.cur.next
}

func (iter *Iterator) Value() interface{} {
  return iter.cur.val
}

func (iter *Iterator) SeekToFirst() {
  iter.cur = iter.head.next
}

