package mq

import "sync"

type mqWapper struct {
	sync.Once
	queue *Queue
	name string
}

func newWrapper(name string) *mqWapper {
	return &mqWapper{
		name: name,
	}
}

func (w *mqWapper) Init() *Queue {
	w.Once.Do(func() {
		w.queue = initQueue(w.name)
	})
	return w.queue
}
