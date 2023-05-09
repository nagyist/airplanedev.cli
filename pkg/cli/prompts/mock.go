package prompts

import "github.com/pkg/errors"

// Mock is a mock Prompter that can be used for testing.
type Mock struct {
	// responses is a list of responses to return in order.
	responses []interface{}
	// idx is the current index of responses.
	idx int
}

var _ Prompter = &Mock{}

// NewMock returns a new Mock Prompter.
func NewMock(responses ...interface{}) *Mock {
	return &Mock{
		responses: responses,
		idx:       0,
	}
}

// Confirm is a mock implementation of Prompter.Confirm. The next response in the list of responses must be a bool.
func (m *Mock) Confirm(question string, o ...Opt) (bool, error) {
	val := m.responses[m.idx]
	m.idx++

	v, ok := val.(bool)
	if !ok {
		return false, errors.New("value must be a bool")
	}

	return v, nil
}

func (m *Mock) ConfirmWithAssumptions(question string, assumeYes, assumeNo bool, o ...Opt) (bool, error) {
	if assumeYes {
		return true, nil
	}

	if assumeNo {
		return false, nil
	}

	return m.Confirm(question, o...)
}

// Input is a mock implementation of Prompter.Input. The next response in the list of responses must be a string.
func (m *Mock) Input(question string, p *string, o ...Opt) error {
	val := m.responses[m.idx]
	m.idx++

	v, ok := val.(string)
	if !ok {
		return errors.New("value must be a string")
	}

	*p = v
	return nil
}
