package pgroup

import (
	"runtime"
	"sync"
	"time"

	"gopkg.in/crast/app.v0/crash"
)

/*
Group is a heterogeneous group of goroutines managed together.
*/
type Group struct {
	mutex    *sync.Mutex
	closers  []func() error
	running  map[*Task]bool
	notify   chan *Task
	stopping bool
	stopchan chan struct{}
	cond     *sync.Cond

	// Optional event handlers that can be set by the user.
	// Event handlers must not be changed once the group has begun running
	// so it is adviseable to set these as soon as creating a group before running Go()

	// FilterError is called when an error occurs during running a runnable started by Go().
	// By default, it's expected to return the error back but it can be used to suppress errors
	// by returning nil; this will suppress the shutdown process from starting.
	FilterError func(err error) error

	// ErrorHandler is called whenever a function run by Go or a closer returns an error value.
	// It is not called if an error is suppressed by FilterError.
	ErrorHandler func(crash.ErrorInfo)

	// PanicHandler is called whenever a function run by Go, GoF, or a closer panics.
	PanicHandler func(crash.PanicInfo)

	DebugHandler func(string, ...interface{})

	// The amount of time, after which, parallel closer operations may end up running in more goroutines.
	// In other words, if some other instance of Wait is
	StopUnblockTime time.Duration
}

func New() *Group {
	mu := &sync.Mutex{}
	g := &Group{
		running:  make(map[*Task]bool),
		notify:   make(chan *Task, 1),
		stopchan: make(chan struct{}, 1),
		mutex:    mu,
		cond:     sync.NewCond(mu),

		StopUnblockTime: 10 * time.Second,

		FilterError:  func(err error) error { return err },
		ErrorHandler: func(crash.ErrorInfo) {},
		PanicHandler: func(crash.PanicInfo) {},
		DebugHandler: func(string, ...interface{}) {},
	}
	g.stopchan <- struct{}{}
	return g
}

// Add a closer to do some sort of cleanup.
// Closers are run last-in first-out.
func (g *Group) AddCloser(closer func() error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.closers = append(g.closers, closer)
}

/*
GoF runs func f in a goroutine.
This is a handy shortcut to running Go when you have a nullary function.

Exactly equivalent to:

    group.Go(func() error {
        f()
        return nil
    })
*/
func (g *Group) GoF(f func()) {
	g.Go(func() error {
		f()
		return nil
	})
}

// Go runs func runnable in a goroutine.
// If the runnable panics, captures the panic and starts the shutdown process.
// If the runnable returns a non-nil error, then also starts the shutdown process.
//
// The returned task can be used for waiting on; it will have already been Started
func (g *Group) Go(runnable func() error) *Task {
	task := &Task{
		group:    g,
		runnable: runnable,
	}
	task.Start()
	return task
}

// Start N copies of the same goroutine.
//
// Basically equivalent to
//     for i := 0; i < n; i++ { group.Go(runnable ) }
func (g *Group) StartN(n int, runnable func() error) {
	for i := 0; i < n; i++ {
		g.Go(runnable)
	}
}

// Wait until all goroutines run with Go complete, and then run all closers in reverse order.
//
// Wait is the key component of the process group. If Wait is not run, then the process group
// will never get a chance to clean itself up and will leak resources.
//
// It is allowable to run Wait simultaneously in multiple goroutines, and when this is done,
// all of the Wait will end at some point shortly after all goroutines have completed.
func (g *Group) Wait() {
	// Drain until there's nothing stopped; allows stoppers to start goroutines if desired.
	for {
		g.drainRunning()
		if !g.waitStop() {
			break
		}
	}
	g.setStopping(false)

	// wake up anyone else who might be blocked on wait
	select {
	case g.notify <- nil:
	default:
	}
}

func (g *Group) drainRunning() {
	g.mutex.Lock()
	l := len(g.running)
	g.mutex.Unlock()

	for l > 0 {
		task, ok := <-g.notify
		if !ok {
			break
		}
		g.mutex.Lock()
		delete(g.running, task)
		l = len(g.running)
		if task != nil {
			g.cond.Broadcast()
		}
		g.mutex.Unlock()
	}
}

// Stop signals the group to begin stopping.
// It fires a goroutine to begin running the closers and returns to its caller immediately.
func (g *Group) Stop() {
	go g.waitStop()
}

func (g *Group) waitStop() bool {
	g.setStopping(true)
	select {
	case _, ok := <-g.stopchan:
		g.debug("Got stop message %v", ok)
		if ok {
			defer g.markDoneStop()
		}
	case <-time.After(g.StopUnblockTime):
		g.debug("timed out waiting for stoppage")
	}
	return g.syncStop()
}

func (g *Group) markDoneStop() {
	select {
	case g.stopchan <- struct{}{}:
		g.debug("returned our token")
		// we woke someone else up
	default:
		g.debug("donestop ran into a full queue?")
		//	// nothing else to do here
	}
}

func (g *Group) syncStop() (closed bool) {
	for {
		closer := g.popCloser()
		if closer == nil {
			break
		} else {
			closed = true
			if err := g.filterError(closer()); err != nil {
				g.triggerError(closer, err)
			}
		}
	}
	return closed
}

// Return true if we're in the stop loop (running closers)
// Needs to acquire a lock, so it is not wise to hit this in a tight loop.
func (g *Group) Stopping() bool {
	g.mutex.Lock()
	result := g.stopping
	g.mutex.Unlock()
	return result
}

func (g *Group) setStopping(val bool) {
	g.mutex.Lock()
	g.stopping = val
	g.mutex.Unlock()
}

// helper to pop the last closer off the stack.
// returns nil if there are no closers remaining.
func (g *Group) popCloser() (closer func() error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	i := len(g.closers) - 1
	if i >= 0 {
		closer = g.closers[i]
		g.closers = g.closers[:i]
	}
	return
}

// Only call FilterError if err != nil
func (g *Group) filterError(err error) error {
	if err != nil {
		err = g.FilterError(err)
	}
	return err
}

func (g *Group) debug(fmt string, v ...interface{}) {
	g.DebugHandler(fmt, v...)
}

func (g *Group) triggerError(runnable func() error, err error) {
	g.ErrorHandler(crash.NewCrashInfo(&crash.CrashData{
		Runnable: runnable,
		Err:      err,
	}))
}

// Task is used to collect results of one single invocation.
type Task struct {
	group    *Group
	err      error
	runnable func() error
	state    state
}

// Err returns the error associated after running this task.
func (t *Task) Err() (err error) {
	t.group.mutex.Lock()
	err = t.err
	t.group.mutex.Unlock()
	return err
}

// SetErr sets the error.
func (t *Task) setErr(err error) {
	t.group.mutex.Lock()
	t.err = err
	t.group.mutex.Unlock()
}

// Failed returns true if the task has failed.
// Will acquire a lock, so do not run this on a tight loop.
func (t *Task) Failed() bool {
	t.group.mutex.Lock()
	result := (t.state == statePanicked || t.err != nil)
	t.group.mutex.Unlock()
	return result
}

// Wait on this task.
func (t *Task) Wait() (err error) {
	t.group.mutex.Lock()
	for t.state == stateNew || t.state == stateRunning || t.group.running[t] {
		t.group.cond.Wait()
	}
	err = t.err
	t.group.mutex.Unlock()
	return err
}

// Start the task. Only call this once!
func (t *Task) Start() {
	t.group.mutex.Lock()
	t.group.running[t] = true
	t.state = stateRunning
	t.group.mutex.Unlock()

	go t.run()
}

func (t *Task) setState(state state) {
	t.group.mutex.Lock()
	t.state = state
	t.group.mutex.Unlock()
}

func (t *Task) run() {
	g := t.group

	defer func() {
		g.notify <- t
		if v := recover(); v != nil {
			t.setState(statePanicked)
			stackbuf := make([]byte, 16*1024)
			i := runtime.Stack(stackbuf, false)
			stackbuf = stackbuf[:i]
			g.PanicHandler(crash.NewCrashInfo(&crash.CrashData{
				Runnable: t.runnable,
				PanicVal: v,
				Stack:    stackbuf,
			}))
		} else {
			t.setState(stateFinished)
		}
	}()

	origError := t.runnable()
	if origError != nil {
		t.setErr(origError)
	}

	if err := g.filterError(origError); err != nil {
		g.triggerError(t.runnable, err)
		g.Stop()
	}
}

type state uint8

const (
	stateNew state = iota
	stateRunning
	stateFinished
	statePanicked
)
