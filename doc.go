/*
Package app runs service components, capturing panics and managing shutdown.

Purpose

When building services in Go, there is usually a number of tasks that are
being done in a single binary. For example, you might launch an HTTP server,
something to send heartbeats to a monitoring service, and something to listen
for change events from a configuration service, and each of them has their own
goroutines.

The biggest headache then with having all these components is managing many
goroutines; the most commonly used solution to that is sync.WaitGroup. However,
if you want to also have shutdown triggered if an individual component fails,
now you start passing state around. Also, issues arise with un-handled panics,
so now all your goroutines need to be wrapped in recover() with handlers.

app is a very small solution designed for navigating that mess.

Features

 * Run a bunch of goroutines, starting them wherever or whenever you want
 * Captures panics without having to propagate a bunch of panic wrapper functions
 * Trigger shutdown events on error, panic, or intentional shutdown
 * your main() will wait for all goroutines to finish.
 * Handles ctrl-C (os.Interrupt) signal and will run closers then.

Basics

Run something in a goroutine:
    app.Go(func() {
        log.Printf("Hello, goroutine!")
    })

    // You can also use errors to initiate shutdown of the app.
    app.Go(func() error{
        time.Sleep(10 * time.Second)
        return errors.New("Error causes shutdown to start!")
    })



Wait for all tasks to finish:

    app.Main() // blocks until all goroutines finish and closers run.


Usage Tips

1. You can run short tasks or even setup kinds of pieces inside an app.Go - it's
just as applicable for short-running things as the long-running app portions.
This lets you parallelize your app startup, for example.

2. app.Go can even be run inside another thing run within app.Go - no need to
start everything from your 'main' goroutine

3. Closer errors primarily are there for symmetry and logging purposes. Use
them if you need them, don't otherwise.

4. app.Main() is actually goroutine-safe; it can be run in multiple places and
all of the instances of Main() will return when the app completes running.

5. os.Exit and facilities which call it (like log.Fatal) will cause your app to
stop running immediately. This means all your cleanup code will never get a
chance to run! Don't use them if you care about clean shutdown.

Notes

Libraries which manage their own go-routines inside a specific scope and want
to control their lifetime should instead use the pgroup sub-package:
https://godoc.org/gopkg.in/crast/app.v0/pgroup

*/
package app
