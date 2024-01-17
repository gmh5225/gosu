package job

import (
	"sync/atomic"
	"time"

	"github.com/can1357/gosu/pkg/util"
)

type TriggerEvery struct {
	Duration util.ParsableDuration `json:"duration"`
}

func (t TriggerEvery) Listen(callback func()) (remove func()) {
	done := &atomic.Bool{}
	go func() {
		for {
			time.Sleep(t.Duration.Duration)
			if done.Load() {
				return
			}
			callback()
		}
	}()
	return func() {
		done.Store(true)
	}
}
func (h *TriggerEvery) UnmarshalInline(text string) (err error) {
	return h.Duration.UnmarshalText([]byte(text))
}
func init() {
	TriggerRegistry.Define("every", TriggerEvery{})
}
