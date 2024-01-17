package job

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/can1357/gosu/pkg/automarshal"
)

type eventSubscription interface {
	Add(*namedEvent)
	Remove()
	Signal()
}

var namedEvents = sync.Map{} // string -> *namedEvent
type namedEvent struct {
	subscribers sync.Map // eventSubscription -> *
}

type asyncEventSubscription struct {
	fn    func()
	event *namedEvent
}

func (sub *asyncEventSubscription) Add(event *namedEvent) {
	sub.event = event
	event.subscribers.Store(sub, struct{}{})
}
func (sub asyncEventSubscription) Remove() {
	evt := sub.event
	if evt == nil {
		return
	}
	evt.subscribers.Delete(sub)
}
func (sub asyncEventSubscription) Signal() {
	go sub.fn()
}

type bufferedEventSubscription struct {
	fn    func()
	event *namedEvent
	queue atomic.Int32
}

func (sub *bufferedEventSubscription) Add(event *namedEvent) {
	sub.event = event
	event.subscribers.Store(sub, struct{}{})
}
func (sub *bufferedEventSubscription) Remove() {
	if sub.queue.Swap(-0x80000000) >= 0 {
		evt := sub.event
		if evt == nil {
			return
		}
		evt.subscribers.Delete(sub)
	}
}
func (sub *bufferedEventSubscription) Signal() {
	if sub.queue.Add(1) == 1 {
		go func() {
			for {
				sub.fn()
				if sub.queue.Add(-1) <= 0 {
					break
				}
			}
		}()
	}
}

func (evt *namedEvent) Signal() {
	evt.subscribers.Range(func(sub, _ any) bool {
		go sub.(eventSubscription).Signal()
		return true
	})
}
func (evt *namedEvent) Listen(sub eventSubscription) (remove func()) {
	evt.subscribers.Store(sub, struct{}{})
	return sub.Remove
}
func getNamedEvent(name string, insert bool) *namedEvent {
	evt, ok := namedEvents.Load(name)
	if !ok {
		if !insert {
			return nil
		}
		actual, _ := namedEvents.LoadOrStore(name, &namedEvent{})
		evt = actual
	}
	return evt.(*namedEvent)
}

func Signal(name string) {
	fmt.Printf("Signaling event %s\n", name)
	if evt := getNamedEvent(name, false); evt != nil {
		evt.Signal()
	}
}
func Listen(name string, callback func()) (cancel func()) {
	return getNamedEvent(name, true).Listen(&asyncEventSubscription{fn: callback})
}
func ListenBuffered(name string, callback func()) (cancel func()) {
	return getNamedEvent(name, true).Listen(&bufferedEventSubscription{fn: callback})
}

type TriggerOn struct {
	Name string `json:"name"`
}
type TriggerOnce struct {
	Name string `json:"name"`
}

func (t TriggerOn) Listen(callback func()) (remove func()) {
	return Listen(t.Name, callback)
}
func (t TriggerOnce) Listen(callback func()) (remove func()) {
	return Listen(t.Name, func() {
		remove()
		callback()
	})
}

func (h *TriggerOn) UnmarshalInline(text string) (err error) {
	h.Name = text
	return
}
func (h *TriggerOnce) UnmarshalInline(text string) (err error) {
	h.Name = text
	return
}
func (h *TriggerOn) WithDefaults(i automarshal.ID) {
	if h.Name == "" {
		h.Name = i.ID
	}
}
func (h *TriggerOnce) WithDefaults(i automarshal.ID) {
	if h.Name == "" {
		h.Name = i.ID
	}
}

func init() {
	TriggerRegistry.Define("on", TriggerOn{})
	TriggerRegistry.Define("once", TriggerOnce{})
}
