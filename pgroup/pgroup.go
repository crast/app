package pgroup

import (
	"runtime"
	"sync"
	"time"

	"gopkg.in/crast/app.v0/crash"
	"gopkg.in/crast/app.v0/multierror"
)

/*
Group is a heterogeneous group of goroutines managed together.
*/
type Group struct {
	lastPid  int
	mutex    sync.Mutex
	closers  []func() error
	running  map[int]bool
	notify   chan int
	stopping bool
	stopchan chan struct{}

	captureErrors bool
	errors        multierror.Errors

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
	StopUnblockTime time.Duration
}

func New() *Group {
	g := &Group{
		running:  make(map[int]bool),
		notify:   make(chan int, 1),
		stopchan: make(chan struct{}, 1),

		StopUnblockTime: 10 * time.Second,

		FilterError:  func(err error) error { return err },
		ErrorHandler: func(crash.ErrorInfo) {},
		PanicHandler: func(crash.PanicInfo) {},
		DebugHandler: func(string, ...interface{}) {},
	}
	g.stopchan <- struct{}{}
	return g
}

// Enables capture of errors, and returns the same group.
// Error capture must be enabled before using with WaitCapture
func (g *Group) EnableCapture() *Group {
	if g.errors == nil {
		g.errors = make(multierror.Errors, 4)
	}
	g.captureErrors = true
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
func (g *Group) Go(runnable func() error) {
	g.mutex.Lock()
	g.lastPid++
	pid := g.lastPid
	g.running[pid] = true
	g.mutex.Unlock()
	go g.run(pid, runnable)
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
	case g.notify <- -1:
	default:
	}
}

// WaitCapture runs wait, and then returns all errors captured during the running of this group.
// Unlike Wait, it is not allowable to run WaitCapture in multiple goroutines.
func (g *Group) WaitCapture() multierror.Errors {
	g.Wait()
	g.mutex.Lock()
	defer g.mutex.Unlock()
	errors := g.errors
	g.errors = nil
	return errors
}

func (g *Group) drainRunning() {
	g.mutex.Lock()
	l := len(g.running)
	g.mutex.Unlock()

	for l > 0 {
		pid, ok := <-g.notify
		if !ok {
			break
		}
		//g.debug("Got completion signal for pid %d", pid)
		g.mutex.Lock()
		delete(g.running, pid)
		l = len(g.running)
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

func (g *Group) run(pid int, runnable func() error) {
	defer func() {
		g.notify <- pid
		if v := recover(); v != nil {
			stackbuf := make([]byte, 16*1024)
			i := runtime.Stack(stackbuf, false)
			stackbuf = stackbuf[:i]
			g.PanicHandler(crash.NewCrashInfo(&crash.CrashData{
				Runnable: runnable,
				PanicVal: v,
				Stack:    stackbuf,
			}))
		}
	}()
	err := g.filterError(runnable())
	if err != nil {
		g.triggerError(runnable, err)
		g.Stop()
	}
}

func (g *Group) triggerError(runnable func() error, err error) {
	g.ErrorHandler(crash.NewCrashInfo(&crash.CrashData{
		Runnable: runnable,
		Err:      err,
	}))
	if g.captureErrors {
		g.mutex.Lock()
		g.errors = append(g.errors, err)
		g.mutex.Unlock()
	}
}
