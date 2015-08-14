// +build !appdebug

package app

/*
Debug printing for app.

This debug function is a no-op unless the library is compiled with the
'appdebug' compiler tag (e.g. "go build -tags appdebug <package>" and such.

This allows the compiler to potentially optimize out the debug call when
this tag isn't there.
*/
func Debug(fmt string, v ...interface{}) {
}
