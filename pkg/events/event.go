package events

import (
	"sync"
	"sync/atomic"
)

type Event[Arg any] struct {
	nextId atomic.Int32
	store  sync.Map
}

func NewEvent[Arg any]() *Event[Arg] {
	return &Event[Arg]{}
}

type EventHandle struct {
	store *sync.Map
	id    int32
}

func (h EventHandle) Off() {
	h.store.Delete(h.id)
}

func (e *Event[Arg]) On(cb func(Arg)) EventHandle {
	handle := EventHandle{store: &e.store, id: e.nextId.Add(1)}
	e.store.Store(handle.id, cb)
	return handle
}
func (e *Event[Arg]) emit(arg Arg, wg *sync.WaitGroup, broadcast bool) {
	if wg != nil {
		e.store.Range(func(key, value any) bool {
			wg.Add(1)
			go func() {
				defer wg.Done()
				value.(func(Arg))(arg)
			}()
			return broadcast
		})
	} else {
		e.store.Range(func(key, value any) bool {
			go value.(func(Arg))(arg)
			return broadcast
		})
	}
}
func (e *Event[Arg]) Emit(arg Arg, wg *sync.WaitGroup) {
	e.emit(arg, wg, true)
}
func (e *Event[Arg]) EmitOne(arg Arg, wg *sync.WaitGroup) {
	e.emit(arg, wg, false)
}
func (e *Event[Arg]) Signal(wg *sync.WaitGroup) {
	var zero Arg
	e.emit(zero, wg, true)
}
func (e *Event[Arg]) SignalOne(wg *sync.WaitGroup) {
	var zero Arg
	e.emit(zero, wg, false)
}

type VoidEvent = Event[struct{}]

func NewVoidEvent() *VoidEvent {
	return NewEvent[struct{}]()
}

type Listener[Arg any] struct {
	C <-chan Arg
	h EventHandle
}

func (l *Listener[Arg]) Stop() {
	l.h.Off()
}

func NewListener[Arg any](e *Event[Arg]) (res *Listener[Arg]) {
	channel := make(chan Arg, 8)
	res = &Listener[Arg]{C: channel}
	res.h = e.On(func(arg Arg) {
		channel <- arg
	})
	return
}

func Listen[Arg any](e *Event[Arg]) <-chan Arg {
	return NewListener[Arg](e).C
}

type VoidListener = Listener[struct{}]
