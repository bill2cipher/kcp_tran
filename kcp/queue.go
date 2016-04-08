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
  entry := q.next
  q.next = entry.next
  entry.next.prev = q
  *q.val.(*uint32)--

  entry.next = nil
  entry.prev = nil
  return entry
}

func (q *Queue) Delete(entry *Queue) {
  entry.next.prev, entry.prev.next = entry.prev, entry.next
  *q.val.(*uint32)--
}
