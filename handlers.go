package app

import (
	"fmt"
	"net"

	"gopkg.in/crast/app.v0/pgroup"
)

type PanicInfo interface {
	pgroup.PanicInfo
}

// Given when we have an error return value from a runnable or a closer.
type ErrorInfo interface {
	pgroup.ErrorInfo
}

// A function to handle panics.
// Can be overridden if desired to provide your own panic responder.
var PanicHandler = func(info PanicInfo) {
	fmt.Printf("[app] Panic recovered: %v\nStack: %s\n", info.PanicVal(), info.Stack())
}

// A function to handle errors.
// Can be overridden if desired to provide your own error responder.
var ErrorHandler = func(info ErrorInfo) {
	fmt.Printf("[app] Got error: %v\n", info.Err())
}

// Filter errors we expect to have that are not really errors.
// Can be overridden to provide your own error filter.
var FilterError = func(err error) error {
	if opErr, ok := err.(*net.OpError); ok && opErr.Op == "accept" {
		if Stopping() && opErr.Err.Error() == "use of closed network connection" {
			Debug("filtered error %v", opErr)
			return nil
		}
	}
	return err
}
