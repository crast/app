package pgroup

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitStopDoesntDeadlock(t *testing.T) {
	assert := assert.New(t)
	var handle int32
	group := New()
	group.Go(stupidWorkFunc(group, 5, &handle))
	group.Go(stupidWorkFunc(group, 10, &handle))
	group.Wait()
	assert.False(group.Stopping())
	group.debug("first waitstop")
	group.waitStop()
	assert.True(group.Stopping())
	group.debug("Second waitstop")
	group.waitStop()
	assert.True(group.Stopping())
	group.setStopping(false)
}

func TestParallelWaitDeadlock(t *testing.T) {
	assert := assert.New(t)
	var handle int32
	group := New()
	group.Go(stupidWorkFunc(group, 20, &handle))
	group.Go(stupidWorkFunc(group, 30, &handle))

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			group.Wait()
		}()
	}
	wg.Wait()
	assert.Equal(int32(50), handle)
	group.running[-1] = true
	group.Go(stupidWorkFunc(group, 10, &handle))
	group.Wait()
	assert.False(group.running[-1])
	assert.Equal(int32(60), handle)
}

///// UTILITIES

func stupidWorkFunc(group *Group, t time.Duration, v *int32) func() error {
	return func() error {
		time.Sleep(t * time.Millisecond)
		group.Go(func() error {
			atomic.AddInt32(v, int32(t))
			return nil
		})
		return nil
	}
}
