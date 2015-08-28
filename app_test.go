package app

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSimpleAppCloses(t *testing.T) {
	var handle int32
	Go(stupidWorkFunc(5, &handle))
	Go(stupidWorkFunc(10, &handle))
	AddCloser(asCloser(func() error {
		stupidWorkFunc(20, &handle)()
		Go(stupidWorkFunc(10, &handle))
		return nil
	}))
	Main()
	assert.Equal(t, int32(45), handle)
}

func TestCloserOrdering(t *testing.T) {
	c := make(chan int, 10)
	buildCloser := func(blah int) asCloser {
		return func() error {
			c <- blah
			return nil
		}
	}
	AddCloser(func() error {
		close(c)
		return nil
	})
	AddCloser(buildCloser(1))
	AddCloser(buildCloser(2))
	AddCloser(buildCloser(3))
	AddCloser(buildCloser(4))
	Main()
	var v []int
	for i := range c {
		v = append(v, i)
	}
	assert.Equal(t, v, []int{4, 3, 2, 1})
}

func TestFilterError(t *testing.T) {
	currentHandler := ErrorHandler
	defer func() {
		ErrorHandler = currentHandler
	}()

	setup := func() chan struct{} {
		stop := make(chan struct{})
		Go(func() error {
			<-stop
			return &net.OpError{Op: "accept", Err: errors.New("use of closed network connection")}
		})
		return stop
	}

	// Phase 1, should error
	stop := setup()
	Go(func() { close(stop) })
	Main()

	// Phase 2, should filter.
	ErrorHandler = func(ErrorInfo) {
		t.FailNow()
	}

	stop = setup()
	AddCloser(func() { close(stop) })
	Go(func() {
		time.Sleep(50 * time.Millisecond)
		Stop()
	})
	Main()
}

type asCloser func() error

func (f asCloser) Close() error {
	return f()
}

func stupidWorkFunc(t time.Duration, v *int32) func() error {
	return func() error {
		time.Sleep(t * time.Millisecond)
		Go(func() error {
			atomic.AddInt32(v, int32(t))
			return nil
		})
		return nil
	}
}
