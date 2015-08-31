package app //  import "gopkg.in/crast/app.v0"

import (
	"net"
	"runtime"
	"sync"
	"time"
)

var lastPid = 0
var mutex sync.Mutex
var closers []func() error
var running = make(map[int]*runstate)
var notify = make(chan int)
var stopping bool
var stopchan chan struct{}

func init() {
	stopchan = make(chan struct{}, 1)
	stopchan <- struct{}{}
}

// Add a closer to do some sort of cleanup.
// Closers are run last-in first-out.
func AddCloser(closer Closeable) {
	mutex.Lock()
	defer mutex.Unlock()
	closers = append(closers, adaptCloser(f))
}

// Run func runnable in a goroutine.
// If the runnable panics, captures the panic and starts the shutdown process.
// If the runnable returns a non-nil error, then also starts the shutdown process.
func Go(f Runnable) {
	runnable := adaptRunnable(f)
	state := &runstate{true}
	mutex.Lock()
	lastPid++
	pid := lastPid
	running[pid] = state
	mutex.Unlock()
	go state.run(pid, runnable)
}

// Run your app until it's complete.
func Main() {
	// Drain until there's nothing stopped; allows stoppers to start goroutines if desired.
	for {
		drainRunning()
		if !waitStop() {
			break
		}
	}
	setStopping(false)
}

func drainRunning() {
	for {
		mutex.Lock()
		l := len(running)
		mutex.Unlock()
		if l == 0 {
			break
		}
		pid, ok := <-notify
		if !ok {
			break
		}
		Debug("Got completion signal for pid %d", pid)
		mutex.Lock()
		delete(running, pid)
		mutex.Unlock()
	}
}

// Signal the app to begin stopping.
// Stop returns to its caller immediately.
func Stop() {
	go waitStop()
}

func waitStop() bool {
	setStopping(true)
	select {
	case _, ok := <-stopchan:
		Debug("Got stop message %v", ok)
		if ok {
			defer markDoneStop()
		}
	case <-time.After(10 * time.Second):
		Debug("timed out waiting for stoppage")
	}
	return syncStop()
}

func markDoneStop() {
	select {
	case stopchan <- struct{}{}:
		Debug("returned our token")
		// we woke someone else up
	default:
		Debug("donestop ran into a full queue?")
		//	// nothing else to do here
	}
}

func syncStop() (closed bool) {
	for {
		closer := popCloser()
		if closer == nil {
			break
		} else {
			closed = true
			if err := filterError(closer()); err != nil {
				ErrorHandler(crashInfo{
					runnable: closer,
					err:      err,
				})
			}
		}
	}
	return closed
}

// Return true if we're in the stop loop (running closers)
// Needs to acquire a lock, so it is not wise to hit this in a tight loop.
func Stopping() bool {
	mutex.Lock()
	result := stopping
	mutex.Unlock()
	return result
}

func setStopping(val bool) {
	mutex.Lock()
	stopping = val
	mutex.Unlock()
}

// helper to pop the last closer off the stack.
// returns nil if there are no closers remaining.
func popCloser() (closer func() error) {
	mutex.Lock()
	defer mutex.Unlock()
	i := len(closers) - 1
	if i >= 0 {
		closer = closers[i]
		closers = closers[:i]
	}
	return
}

// Shortcut for things which look like servers.
// Adds the listener as a closer, and then runs Go on the serveable.
func Serve(l net.Listener, server Serveable) {
	AddCloser(l)
	Go(func() error { return server.Serve(l) })
}

type Serveable interface {
	Serve(net.Listener) error
}

type runstate struct {
	running bool
}

func (r *runstate) run(pid int, runnable func() error) {
	defer func() {
		r.running = false
		notify <- pid
		if v := recover(); v != nil {
			stackbuf := make([]byte, 16*1024)
			i := runtime.Stack(stackbuf, false)
			stackbuf = stackbuf[:i]
			PanicHandler(crashInfo{
				runnable: runnable,
				panicVal: v,
				stack:    stackbuf,
			})
		}
	}()
	err := filterError(runnable())
	if err != nil {
		ErrorHandler(crashInfo{
			runnable: runnable,
			err:      err,
		})
		Stop()
	}
}
