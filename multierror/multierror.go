package multierror

import "bytes"

type Errors []error

func (e *Errors) Add(err error) {
	if err != nil {
		*e = append(*e, err)
	}
}

/**
 * Resolve to either no error (nil), one error, or multiple errors.
 */
func (e Errors) Resolve() error {
	switch len(e) {
	case 0:
		return nil
	case 1:
		return e[0]
	default:
		return e
	}
}

func (e Errors) Error() string {
	var buf bytes.Buffer
	for i, err := range e {
		buf.WriteString(err.Error())
		if i != len(e)-1 {
			buf.WriteString("; ")
		}
	}
	return buf.String()
}
