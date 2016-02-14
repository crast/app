package app

import (
	"os"
	"os/signal"
)

var sigchan = make(chan os.Signal, 2)

func sigListener() {
	for s := range sigchan {
		Debug("Got Signal %#v", s)
		Stop()
	}
}

// AddStopSignal adds a new signal to listen to that will trigger shutdown.
// By default, we already listen on os.Interrupt, but you can add some of your own
// such as os-specific ones from the syscall package.
func AddStopSignal(sig os.Signal) {
	signal.Notify(sigchan, sig)
}

func init() {
	go sigListener()
	AddStopSignal(os.Interrupt)
}
