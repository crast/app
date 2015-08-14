package app

import (
	"fmt"
)

// Defines the types which can be closed.
// These types include:
//   Closer
//   func()
//   func() error
type Closeable interface{}

// Defines the types which can be run.
// These types include:
//   func()
//   func() error
//   RunCloser
type Runnable interface{}

type Closer interface {
	Close() error
}

type RunCloser interface {
	Closer
	Run() error
}

func adaptRunnable(r interface{}) func() error {
	if rc, ok := r.(RunCloser); ok {
		AddCloser(rc.Close)
		return rc.Run
	} else if f, ok := r.(func() error); ok {
		return f
	} else if bareFunc, ok := r.(func()); ok {
		return adaptBareFunc(bareFunc)
	} else {
		panic(fmt.Errorf("Value %#v is not a valid runnable", r))
	}
}

func adaptCloser(c Closeable) func() error {
	if f, ok := c.(func() error); ok {
		return f
	} else if closer, ok := c.(Closer); ok {
		return closer.Close
	} else if bareFunc, ok := c.(func()); ok {
		return adaptBareFunc(bareFunc)
	} else {
		panic(fmt.Errorf("Value %#v is not a valid closeable", c))
	}
}

func adaptBareFunc(bareFunc func()) func() error {
	return func() error {
		bareFunc()
		return nil
	}
}
