# app
Run your go app, with signals, shutdown, closing, etc.

## Basics

* Manages goroutines and sets up panic handlers for them
* Runs shutdown tasks when either:
  - all goroutines finish naturally
  - one goroutine errors or panics
  - receives a `SIGINT`
* Blocks until all goroutines and shutdown tasks are complete.

Godoc: http://godoc.org/gopkg.in/crast/app.v0

## Example

```go

import (
    "net"
    "os"

    "gopkg.in/crast/app.v0"
)

func main() {
    someResource := os.Open("filepath")
    app.AddCloser(someResource)
    app.Go(someTask)
    app.Go(func() error {
        // do something
        return nil
    })
    lis, _ := net.Listen("tcp", ":80")
    app.Serve(lis, buildServer())

    // wait till all things finish running or SIGINT happens.
    app.Main()
}

func someTask() error {
    // something
}

func buildServer() *http.Server {
    return &http.Server{...}
}
```