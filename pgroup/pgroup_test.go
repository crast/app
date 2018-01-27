package pgroup

import (
	"errors"
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
	group.running[nil] = true
	group.Go(stupidWorkFunc(group, 10, &handle))
	group.Wait()
	assert.False(group.running[nil])
	assert.Equal(int32(60), handle)
}

func TestTaskWait(t *testing.T) {
	assert := assert.New(t)
	var handle int32
	group := New()
	simpleWork := func(t time.Duration) *Task {
		return group.Go(func() error {
			time.Sleep(t * time.Millisecond)
			atomic.AddInt32(&handle, int32(t))
			return nil
		})
	}
	task1 := simpleWork(20)
	task2 := simpleWork(30)
	group.GoF(func() {
		time.Sleep(150 * time.Millisecond)
		simpleWork(40)
	})
	go group.Wait()
	task1.Wait()
	assert.Equal(stateFinished, task1.state)
	assert.NotEqual(stateNew, task2.state)
	task2.Wait()
	assert.Equal(int32(50), handle)
	group.Wait()
	assert.Equal(int32(90), handle)
}

func TestTaskResults(t *testing.T) {
	assert := assert.New(t)

	group := New()
	task := group.Go(func() error {
		time.Sleep(100 * time.Millisecond)
		return errors.New("The Error")
	})
	go group.Wait()
	task.Wait()
	result := task.Err()
	assert.Equal("The Error", result.Error())
	assert.Equal(true, task.Failed())
	assert.Equal("The Error", task.Err().Error())
	assert.Exactly(result, task.Err())
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
