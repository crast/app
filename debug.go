// +build appdebug

package app

import (
	"log"
)

func Debug(fmt string, v ...interface{}) {
	log.Printf(fmt, v...)
}
