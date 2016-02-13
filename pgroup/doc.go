/*
Package pgroup lets you manage a heterogeneous 'group' of goroutines as a single unit.

This is a very similar concept to the app framework except dumbed down a bit:
no automatic conversion of runnables, no built-in signal handling,
and waiting on a group waits on only those goroutines.

Rationale

Group offers these advantages over sync.WaitGroup based goroutine running:

 - Can add goroutines whenever; no race condition on starting
   (see caveat in https://golang.org/pkg/sync/#WaitGroup.Add )
 - No manual management of counting goroutines launched.
 - Supports registering shutdown handlers to do cleanup and early shutdown scenarios.
 - Captures panics in goroutines without you having to write a bunch of panic handlers.
 - Will run shutdown in a last-in, first-out manner when either a panic/error occurs, or
   when all goroutines are complete.
*/
package pgroup
