package pgroup

type PanicInfo interface {
	Task() *Task

	// PanicVal is the actual value that was passed to the call to panic().
	PanicVal() interface{}
	// Stack is a text representation of the stack caught by panic handler.
	Stack() []byte
}

// Given when we have an error return value from a runnable or a closer.
type ErrorInfo interface {
	Task() *Task

	// The error
	Err() error
}

type panicData struct {
	task     *Task
	panicVal interface{}
	stack    []byte
}

func (p *panicData) Task() *Task           { return p.task }
func (p *panicData) PanicVal() interface{} { return p.PanicVal }
func (p *panicData) Stack() []byte         { return p.stack }

type errData struct {
	task *Task
	err  error
}

func (e errData) Task() *Task { return e.task }
func (e errData) Err() error  { return e.err }
