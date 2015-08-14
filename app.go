package app //  import "gopkg.in/crast/app.v0"

import (
	"fmt"
	"net"
	"runtime"
	"sync"
)

var lastPid = 0
var mutex sync.Mutex
var closers []func() error
var running = make(map[int]*runstate)
var notify = make(chan int)

// Add a closer to do some sort of cleanup.
// Closers are run last-in first-out.
func AddCloser(closer Closeable) {
	mutex.Lock()
	defer mutex.Unlock()
	closers = append(closers, adaptCloser(closer))
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
		if !syncStop() {
			break
		}
	}
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
	go syncStop()
}

func syncStop() bool {
	closed := false
	running := true
	for running {
		var closer func() error
		mutex.Lock()
		i := len(closers) - 1
		if i < 0 {
			running = false
		} else {
			closer = closers[i]
			closers = closers[:i]
		}
		mutex.Unlock()
		if closer != nil {
			closed = true
			closer()
		}
	}
	return closed
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
			PanicHandler(v, stackbuf)
		}
	}()
	err := runnable()
	if err != nil {
		ErrorHandler(runnable, err)
		Stop()
	}
}

// A function to handle panics.
// Can be overridden if desired to provide your own panic responder.
var PanicHandler = func(panicVal interface{}, stack []byte) {
	fmt.Printf("Panic recovered: %v\nStack: %s\n", panicVal, stack)
}

var ErrorHandler = func(runnable func() error, err error) {
	fmt.Printf("Got error: %v\n", err)
}
