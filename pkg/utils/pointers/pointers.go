package pointers

import "time"

// String returns a pointer to a string - nil if string is empty.
func String(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ToString returns a string from a pointer - empty if pointer is nil.
func ToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Bool returns a pointer to a bool. The pointer is never nil.
func Bool(b bool) *bool {
	return &b
}

// ToBool returns a bool from a pointer - false if pointer is nil.
func ToBool(s *bool) bool {
	if s == nil {
		return false
	}
	return *s
}

// Int returns a pointer to an int. Never nil.
func Int(i int) *int {
	return &i
}

// ToInt returns an int from a pointer - zero if pointer is nil.
func ToInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// Int64 returns a pointer to an int64. Never nil.
func Int64(i int64) *int64 {
	return &i
}

// ToInt64 returns an int64 from a pointer - zero if pointer is nil.
func ToInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

// Time returns a pointer to a time. Never nil.
func Time(t time.Time) *time.Time {
	return &t
}

// ToTime returns a time from a pointer - zero if pointer is nil.
func ToTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
