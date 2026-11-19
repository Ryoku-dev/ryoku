package pubsub

import (
	"sync"
)

type PubSub[T any] struct {
	mu          sync.Mutex
	next        chan item[T]
	closed      bool
	subscribers int
}

type item[T any] struct {
	value T
	next  chan item[T]
}

type Subscription[T any] struct {
	next   chan item[T]
	Values chan T
	mu     sync.Mutex
	closed chan struct{}
	ps     *PubSub[T]
}

func (me *PubSub[T]) init() {
	me.next = make(chan item[T], 1)
}

func (me *PubSub[T]) lazyInit() {
	me.mu.Lock()
	defer me.mu.Unlock()
	if me.closed {
		return
	}
	if me.next == nil {
		me.init()
	}
}

func (me *PubSub[T]) Publish(v T) {
	me.mu.Lock()
	defer me.mu.Unlock()
	// With no subscribers there's nothing to deliver to, and a later Subscribe resumes from the
	// current me.next, so the value would be dropped regardless. Skip the channel allocation and send.
	// A subscriber always lazyInits me.next before incrementing the count, so me.next is non-nil here.
	if me.closed || me.subscribers == 0 {
		return
	}
	next := make(chan item[T], 1)
	me.next <- item[T]{v, next}
	me.next = next
}

// NumSubs returns the number of active subscribers.
func (me *PubSub[T]) NumSubs() int {
	me.mu.Lock()
	defer me.mu.Unlock()
	return me.subscribers
}

func (me *Subscription[T]) Close() {
	me.mu.Lock()
	defer me.mu.Unlock()
	select {
	case <-me.closed:
	default:
		close(me.closed)
		me.ps.mu.Lock()
		me.ps.subscribers--
		me.ps.mu.Unlock()
	}
}

func (me *Subscription[T]) runner() {
	defer close(me.Values)
	for {
		select {
		case i, ok := <-me.next:
			if !ok {
				me.Close()
				return
			}
			// Send the value back into the channel for someone else. This
			// won't block because the channel has a capacity of 1, and this
			// is currently the only copy of this value being sent to this
			// channel.
			me.next <- i
			// The next value comes from the channel given to us by the value
			// we just got.
			me.next = i.next
			select {
			case me.Values <- i.value:
			case <-me.closed:
				return
			}
		case <-me.closed:
			return
		}
	}
}

func (me *PubSub[T]) Subscribe() (ret *Subscription[T]) {
	me.lazyInit()
	ret = &Subscription[T]{
		closed: make(chan struct{}),
		Values: make(chan T),
		ps:     me,
	}
	me.mu.Lock()
	ret.next = me.next
	me.subscribers++
	me.mu.Unlock()
	go ret.runner()
	return
}

func (me *PubSub[T]) Close() {
	me.mu.Lock()
	defer me.mu.Unlock()
	if me.closed {
		return
	}
	if me.next != nil {
		close(me.next)
	}
	me.closed = true
}
