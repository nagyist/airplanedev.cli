package dev

import (
	"fmt"
	"math"
	"sync"
)

// TODO: Move to lib package so it can be shared among airplane services.
type IDGenerator struct {
	mu sync.Mutex
	id int
}

// Next generates an incrementing string (sortable alphanumeric) from 000000 to 999999
// Rolls over back to 000000 once it hits 999999. Guaranteed to be unique per timestamp if
// called < 1 million times for a given timestamp.
func (i *IDGenerator) Next() string {
	i.mu.Lock()
	nextID := i.id + 1
	digits := 6
	if nextID >= int(math.Pow10(digits)) {
		nextID = 0
	}
	i.id = nextID
	i.mu.Unlock()

	fmtString := fmt.Sprintf("%%0%dd", digits)
	return fmt.Sprintf(fmtString, nextID)
}
