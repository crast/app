package app

import (
	"fmt"
	"os"
	"os/signal"
)

var sigchan chan os.Signal

func sigListener() {
	for s := range sigchan {
		fmt.Printf("Got Signal %#v", s)
		Stop()
	}
}

func init() {
	sigchan = make(chan os.Signal, 2)
	go sigListener()
	signal.Notify(sigchan, os.Interrupt)
}
