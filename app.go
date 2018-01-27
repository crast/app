package app //  import "gopkg.in/crast/app.v0"

import (
	"net"

	"gopkg.in/crast/app.v0/pgroup"
)

var appGroup *pgroup.Group

func init() {
	appGroup = newAppGroup()
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

// ChildGroup returns a new group configured to have handlers
// that will send errors to the app handlers, and stop when the app stops.
//
// The group will not wait on the child unless it's explicitly requested
func ChildGroup() *pgroup.Group {
	group := newAppGroup()
	AddCloser(group.Stop)
	return group
}

// Root returns the process group underlying the app
//
// Just like with pgroup, making modifications to this group
// runs the same risk of race conditions unless this is done
// before anything happens to the group.
func Root() *pgroup.Group {
	return appGroup
}

// newAppGroup makes a group with all the handlers pointing at
// the "app" versions of the handlers so we can use the package
// handlers even if they change.
func newAppGroup() *pgroup.Group {
	group := pgroup.New()
	group.FilterError = func(err error) error { return FilterError(err) }
	group.ErrorHandler = func(info pgroup.ErrorInfo) { ErrorHandler(info) }
	group.PanicHandler = func(info pgroup.PanicInfo) { PanicHandler(info) }
	group.DebugHandler = Debug
	return group
}
