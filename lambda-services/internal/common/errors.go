package common

import "strings"

type MultiError struct {
	errors []error
}

func (e *MultiError) Error() string {
	var messages []string
	for _, err := range e.errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

func (e *MultiError) Add(err error) {
	e.errors = append(e.errors, err)
}

func (e *MultiError) Get() error {
	if len(e.errors) > 0 {
		return e
	} else {
		return nil
	}
}
