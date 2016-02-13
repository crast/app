package pgroup

import (
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
