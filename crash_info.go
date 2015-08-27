package app

import (
	"fmt"
)

// The value given when we have a panic in a runnable.
type PanicInfo interface {
	// The function we were running.
	Runnable() func() error
	// The actual value that comes from panic().
	PanicVal() interface{}
	// The text representation of the stack caught by panic handler.
	Stack() []byte
}

// Given when we have an error return value from a runnable or a closer.
type ErrorInfo interface {
	// The function we were running.
	Runnable() func() error
	// The e
	Err() error
}

type crashInfo struct {
	runnable func() error
	err      error
	panicVal interface{}
	stack    []byte
}

func (c crashInfo) Runnable() func() error {
	return c.runnable
}

func (c crashInfo) Err() error {
	return c.err
}

func (c crashInfo) PanicVal() interface{} {
	return c.panicVal
}

func (c crashInfo) Stack() []byte {
	return c.stack
}

// A function to handle panics.
// Can be overridden if desired to provide your own panic responder.
var PanicHandler = func(info PanicInfo) {
	fmt.Printf("Panic recovered: %v\nStack: %s\n", info.PanicVal(), info.Stack())
}

// A function to handle errors.
// Can be overridden if desired to provide your own error responder.
var ErrorHandler = func(info ErrorInfo) {
	fmt.Printf("Got error: %v\n", info.Err())
}
