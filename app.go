package app //  import "gopkg.in/crast/app.v0"

import (
	"net"

	"gopkg.in/crast/app.v0/crash"
	"gopkg.in/crast/app.v0/pgroup"
)

var appGroup *pgroup.Group

func init() {
	appGroup = pgroup.New()
	appGroup.FilterError = func(err error) error { return FilterError(err) }
	appGroup.ErrorHandler = func(info crash.ErrorInfo) { ErrorHandler(info) }
	appGroup.PanicHandler = func(info crash.PanicInfo) { PanicHandler(info) }
	appGroup.DebugHandler = Debug
}

// Add a closer to do some sort of cleanup.
// Closers are run last-in first-out.
func AddCloser(closer Closeable) {
	appGroup.AddCloser(adaptCloser(closer))
}

// Run func runnable in a goroutine.
// If the runnable panics, captures the panic and starts the shutdown process.
// If the runnable returns a non-nil error, then also starts the shutdown process.
func Go(f Runnable) {
	runnable := adaptRunnable(f)
	appGroup.Go(runnable)
}

// Run your app until it's complete.
func Main() {
	appGroup.Wait()
}

// Signal the app to begin stopping.
// Stop returns to its caller immediately.
func Stop() {
	appGroup.Stop()
}

// Return true if we're in the stop loop (running closers)
// Needs to acquire a lock, so it is not wise to hit this in a tight loop.
func Stopping() bool {
	return appGroup.Stopping()
}

// Shortcut for things which look like servers.
// Adds the listener as a closer, and then runs Go on the serveable.
func Serve(l net.Listener, server Serveable) {
	AddCloser(l)
	appGroup.Go(func() error { return server.Serve(l) })
}

type Serveable interface {
	Serve(net.Listener) error
}
