package task

import (
	"context"
)

type PipeMode interface {
	serialize(index int, count int) (ok bool)
	end(err error, cancel context.CancelCauseFunc)
	String() string
}

// pipe:parallel runs all subtasks in parallel, and cancels the rest if any of them fails.

type PipeParallel struct{}

func (PipeParallel) String() string { return "parallel" }

func (PipeParallel) serialize(index int, count int) bool {
	return false
}
func (PipeParallel) end(err error, cancel context.CancelCauseFunc) {
	if err != nil {
		cancel(err)
	}
}

// pipe:ordered runs all subtasks one after another, and cancels the rest if any of them fails.
type PipeOrdered struct{}

func (PipeOrdered) String() string { return "ordered" }
func (PipeOrdered) serialize(index int, count int) bool {
	return true
}
func (PipeOrdered) end(err error, cancel context.CancelCauseFunc) {
	if err != nil {
		cancel(err)
	}
}

// pipe:race runs all subtasks in parallel, and cancels the rest if any of them succeeds or fails.
type PipeRace struct{}

func (PipeRace) String() string { return "race" }
func (PipeRace) serialize(index int, count int) bool {
	return false
}
func (PipeRace) end(err error, cancel context.CancelCauseFunc) {
	cancel(err)
}

// pipe:any runs all subtasks in parallel, and cancels the rest if any of them succeeds.
type PipeAny struct{}

func (PipeAny) String() string { return "any" }
func (PipeAny) serialize(index int, count int) bool {
	return false
}
func (PipeAny) end(err error, cancel context.CancelCauseFunc) {
	if err == nil {
		cancel(nil)
	}
}

// PipeModes is a list of all pipe modes.
var PipeModes = [...]PipeMode{
	PipeOrdered{},
	PipeParallel{},
	PipeRace{},
	PipeAny{},
}
