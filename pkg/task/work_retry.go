package task

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/can1357/gosu/pkg/util"
	"github.com/samber/lo"
)

const retryTickDead = 0xffffffff

var ErrNonRetriable = errors.New("non-retriable error")

func NonRetriable(err error) error {
	return fmt.Errorf("%w: %v", ErrNonRetriable, err)
}

type retryWorker struct {
	*workerBase
	context.Context                            // The context for the retrier.
	cancel          context.CancelCauseFunc    //
	must            atomic.Pointer[mustWorker] // The underlying work.
	retryState      atomic.Uint64              // Number of errors so far encountered && tick
	retryCancel     chan struct{}              // The channel to cancel the retrier.
	status          Status                     // The current status (if alive).
}

func newRetryWorker(m *workerBase) (w *retryWorker) {
	w = &retryWorker{workerBase: m, retryCancel: make(chan struct{}, 1)}
	w.Context, w.cancel = util.WithCancelOrOk(m)
	return
}

// Implement work interface.
func (retry *retryWorker) Inspect() (r Report) {
	if m := retry.must.Load(); m != nil {
		r = m.Inspect()
	}
	return
}
func (retry *retryWorker) Stop() {
	retry.disableRetries()
	if m := retry.must.Load(); m != nil {
		m.Stop()
	}
}
func (retry *retryWorker) Kill() {
	retry.disableRetries()
	if m := retry.must.Load(); m != nil {
		m.Kill()
	}
}
func (retry *retryWorker) Traverse(fn func(Worker) bool) {
	if m := retry.must.Load(); m != nil {
		m.Traverse(fn)
	}
}

// Retry state.
func unpackErrorState(state uint64) (tick uint32, counter uint32) {
	return uint32(state), uint32(state >> 32)
}
func packErrorState(tick uint32, counter uint32) uint64 {
	return uint64(tick) | (uint64(counter) << 32)
}
func (retry *retryWorker) disableRetries() {
	for {
		expected := retry.retryState.Load()
		tick, counter := unpackErrorState(expected)
		if tick == retryTickDead {
			return
		}
		newState := packErrorState(retryTickDead, counter)
		if retry.retryState.CompareAndSwap(expected, newState) {
			close(retry.retryCancel)
			return
		}
	}
}
func (retry *retryWorker) retriable(err error) bool {
	if err != nil {
		if errors.Is(err, ErrNonRetriable) {
			return false
		}
	} else {
		if !retry.options.RetrySuccess {
			return false
		}
	}
	expected := retry.retryState.Load()
	tick := uint32(expected)
	return tick != retryTickDead
}
func (retry *retryWorker) tryRetry(err error) bool {
	if retry.Context.Err() != nil {
		return false
	}
	if !retry.retriable(err) {
		return false
	}

	// Acquire the retry state, and increment the counter.
	var counter, tick uint32
	rate := retry.options.RetryLimit
	for {
		expected := retry.retryState.Load()
		tick, counter = unpackErrorState(expected)
		if tick == retryTickDead {
			return false
		}
		cur := rate.Ticks()
		if tick != cur {
			tick = cur
			counter = 0
		}
		counter++
		newState := packErrorState(tick, counter)
		if retry.retryState.CompareAndSwap(expected, newState) {
			if counter > rate.Count {
				return false
			}
			break
		}
	}

	// Calculate the retry wait time.
	t := float64(retry.options.RetryBackoff.Duration)
	r := float64(counter-1) / float64(rate.Count)
	r = max(min(r, 1.0), 0.0)
	t = t * (0.5 + (r * r)) * retry.options.RetryBackoffScale
	wait := time.Duration(t)
	retry.Logger().Printf("Retrying in %s (%d/%d), error: %v", wait, counter, rate.Count, err)

	retry.status = Retrying
	select {
	// Retrier is cancelled.
	case <-retry.Done():
		retry.status = Idle
		return false
	case <-retry.retryCancel:
		retry.status = Idle
		return false
	// OK to retry!
	case <-time.After(wait):
		return true
	}
}

func (retry *retryWorker) Status() error {
	if retry.Err() != nil {
		c := util.Cause(retry)
		switch c {
		case nil:
			return Complete
		default:
			return c
		}
	} else if m := retry.must.Load(); m != nil && retry.status.IsAlive() {
		return m.Status()
	} else {
		return retry.status
	}
}
func (retry *retryWorker) Run() (err error) {
	defer func() {
		retry.cancel(err)
	}()

	for {
		// Create the work instance and store it.
		retry.status = Starting
		work := newMustWorker(retry.workerBase)
		retry.must.Store(work)

		// Wait for the work to end.
		retry.status = Running
		select {
		case <-retry.Done():
			err = util.Cause(retry)
			return
		case <-work.Done():
			err = util.Cause(work)
		case err = <-lo.Async(work.Run):
		}
		retry.status = work.status

		// If error is not retriable, exit.
		if !retry.tryRetry(err) {
			break
		}
	}
	retry.status = Idle
	return
}
