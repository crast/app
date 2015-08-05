package app

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSimpleAppCloses(t *testing.T) {
	var handle int32
	Go(stupidWorkFunc(5, &handle))
	Go(stupidWorkFunc(10, &handle))
	AddCloser(adaptCloser(func() error {
		stupidWorkFunc(20, &handle)()
		Go(stupidWorkFunc(10, &handle))
		return nil
	}))
	Main()
	assert.Equal(t, 45, handle)
}

func TestCloserOrdering(t *testing.T) {
	c := make(chan int, 10)
	buildCloser := func(blah int) adaptCloser {
		return func() error {
			c <- blah
			return nil
		}
	}
	AddCloser(adaptCloser(func() error {
		close(c)
		return nil
	}))
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

type adaptCloser func() error

func (f adaptCloser) Close() error {
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
