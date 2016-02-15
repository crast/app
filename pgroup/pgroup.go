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
	lastPid  int
	mutex    sync.Mutex
	closers  []func() error
	running  map[int]*runstate
	notify   chan int
	stopping bool
	stopchan chan struct{}

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
}

func New() *Group {
	g := &Group{
		running:      make(map[int]*runstate),
		notify:       make(chan int, 1),
		stopchan:     make(chan struct{}, 1),
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
func (g *Group) Go(runnable func() error) {
	state := &runstate{g, true}
	g.mutex.Lock()
	g.lastPid++
	pid := g.lastPid
	g.running[pid] = state
	g.mutex.Unlock()
	go state.run(pid, runnable)
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

// Run your app until it's complete.
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

func (g *Group) drainRunning() {
	for {
		g.mutex.Lock()
		l := len(g.running)
		g.mutex.Unlock()
		if l == 0 {
			break
		}
		pid, ok := <-g.notify
		if !ok {
			break
		}
		//g.debug("Got completion signal for pid %d", pid)
		g.mutex.Lock()
		delete(g.running, pid)
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
	case <-time.After(10 * time.Second):
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
				g.ErrorHandler(crash.NewCrashInfo(&crash.CrashData{
					Runnable: closer,
					Err:      err,
				}))
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

type runstate struct {
	group   *Group
	running bool
}

func (r *runstate) run(pid int, runnable func() error) {
	defer func() {
		r.running = false
		r.group.notify <- pid
		if v := recover(); v != nil {
			stackbuf := make([]byte, 16*1024)
			i := runtime.Stack(stackbuf, false)
			stackbuf = stackbuf[:i]
			r.group.PanicHandler(crash.NewCrashInfo(&crash.CrashData{
				Runnable: runnable,
				PanicVal: v,
				Stack:    stackbuf,
			}))
		}
	}()
	err := r.group.filterError(runnable())
	if err != nil {
		r.group.ErrorHandler(crash.NewCrashInfo(&crash.CrashData{
			Runnable: runnable,
			Err:      err,
		}))
		r.group.Stop()
	}
}
