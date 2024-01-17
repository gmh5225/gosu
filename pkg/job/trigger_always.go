package job

import (
	"sync/atomic"
)

type TriggerAlways struct{}
type TriggerNever struct{}

func (t TriggerNever) Listen(callback func()) (remove func()) {
	return func() {}
}
func (t TriggerAlways) Listen(callback func()) (remove func()) {
	done := &atomic.Bool{}
	go func() {
		for !done.Load() {
			callback()
		}
	}()
	return func() {
		done.Store(true)
	}
}

func (h *TriggerAlways) UnmarshalInline(text string) (err error) { return nil }
func (h *TriggerNever) UnmarshalInline(text string) (err error)  { return nil }

func init() {
	TriggerRegistry.Define("always", TriggerAlways{})
	TriggerRegistry.Define("never", TriggerNever{})
}
