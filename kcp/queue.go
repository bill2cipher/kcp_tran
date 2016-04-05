package kcp

import (
  "container/ring"
)

type Queue *ring.Ring

func (q *Queue) Push(val interface{}) {
  
}