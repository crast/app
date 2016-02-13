package crash

// PanicInfo describes what happens when we have a panic in a runnable.
type PanicInfo interface {
	// Runnable is the function we were running.
	Runnable() func() error
	// PanicVal is the actual value that was passed to the call to panic().
	PanicVal() interface{}
	// Stack is a text representation of the stack caught by panic handler.
	Stack() []byte
}

// Given when we have an error return value from a runnable or a closer.
type ErrorInfo interface {
	// The function we were running.
	Runnable() func() error
	// The e
	Err() error
}

type CrashData struct {
	Runnable func() error
	Err      error
	PanicVal interface{}
	Stack    []byte
}

func NewCrashInfo(data *CrashData) CrashInfo {
	return CrashInfo{data: data}
}

// CrashInfo can be used to satisfy either ErrorInfo or PanicInfo
type CrashInfo struct {
	data *CrashData
}

func (c CrashInfo) Runnable() func() error {
	return c.data.Runnable
}

func (c CrashInfo) Err() error {
	return c.data.Err
}

func (c CrashInfo) PanicVal() interface{} {
	return c.data.PanicVal
}

func (c CrashInfo) Stack() []byte {
	return c.data.Stack
}
