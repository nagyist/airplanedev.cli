package pointers

// String returns a pointer to a string - nil if string is empty
func String(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ToString returns a string from a pointer - empty if pointer is nil
func ToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func Bool(b bool) *bool {
	return &b
}
