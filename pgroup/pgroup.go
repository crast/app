/*
Manage a heterogeneous 'group' of goroutines as a single unit.

This is a very similar concept to the app framework except dumbed down a bit:
no automatic conversion of runnables, no built-in signal handling,
and waiting on a group waits on only those goroutines.

This is NOT a stable API currently, and lacking in a number of bits,
right now it's basically copypasta from the app framework for the
purpose of testing/understanding something.
*/
package pgroup

import (
	"log"
	"runtime"
	"sync"
	"time"
)

type Group struct {
	lastPid  int
	mutex    sync.Mutex
	closers  []func() error
	running  map[int]*runstate
	notify   chan int
	stopping bool
	stopchan chan struct{}

	// things people can override
	FilterError func(err error) error
}

func NewGroup() *Group {
	g := &Group{
		running:     make(map[int]*runstate),
		notify:      make(chan int),
		stopchan:    make(chan struct{}, 1),
		FilterError: func(err error) error { return err },
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

// Run func runnable in a goroutine.
// If the runnable panics, captures the panic and starts the shutdown process.
func (g *Group) GoF(f func()) {
	g.Go(func() error {
		f()
		return nil
	})
}

// Run func runnable in a goroutine.
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
		//Debug("Got completion signal for pid %d", pid)
		g.mutex.Lock()
		delete(g.running, pid)
		g.mutex.Unlock()
	}
}

// Signal the app to begin stopping.
// Stop returns to its caller immediately.
func (g *Group) Stop() {
	go g.waitStop()
}

func (g *Group) waitStop() bool {
	g.setStopping(true)
	select {
	case _, ok := <-g.stopchan:
		//Debug("Got stop message %v", ok)
		if ok {
			defer g.markDoneStop()
		}
	case <-time.After(10 * time.Second):
		Debug("timed out waiting for stoppage")
	}
	return g.syncStop()
}

func (g *Group) markDoneStop() {
	select {
	case g.stopchan <- struct{}{}:
		Debug("returned our token")
		// we woke someone else up
	default:
		Debug("donestop ran into a full queue?")
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
				//TODO
				/*ErrorHandler(crashInfo{
					runnable: closer,
					err:      err,
				})*/
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

/*
// Shortcut for things which look like servers.
// Adds the listener as a closer, and then runs Go on the serveable.
func Serve(l net.Listener, server Serveable) {
	AddCloser(l)
	Go(func() error { return server.Serve(l) })
}

type Serveable interface {
	Serve(net.Listener) error
}
*/

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
			// TODO
			/*PanicHandler(crashInfo{
				runnable: runnable,
				panicVal: v,
				stack:    stackbuf,
			})*/
		}
	}()
	err := r.group.filterError(runnable())
	if err != nil {
		// TODO
		/*ErrorHandler(crashInfo{
			runnable: runnable,
			err:      err,
		})*/
		r.group.Stop()
	}
}

func Debug(fmt string, v ...interface{}) {
	log.Printf(fmt, v...)
}
