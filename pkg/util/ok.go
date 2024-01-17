package util

import (
	"context"
	"fmt"
)

var ErrOk = fmt.Errorf("done")

func OrOk(e error) error {
	if e == nil {
		return ErrOk
	}
	return e
}
func NotOk(e error) error {
	if e == ErrOk {
		return nil
	}
	return e
}

type cancelOrOk struct {
	context.Context
	cancel context.CancelCauseFunc
}

func (c *cancelOrOk) Cancel(err error) {
	if err == nil {
		err = ErrOk
	}
	c.cancel(err)
}

func Cause(c context.Context) (err error) {
	if cause := context.Cause(c); cause != nil {
		err = NotOk(cause)
	}
	return
}

func WithCancelOrOk(ctx context.Context) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(ctx)
	result := &cancelOrOk{Context: ctx, cancel: cancel}
	return result, result.Cancel
}
