package pointers

import "time"

// String returns a pointer to a string - nil if string is empty.
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

func Int64(i int64) *int64 {
	return &i
}

func ToInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func Time(t time.Time) *time.Time {
	return &t
}

func ToTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
