package dev_errors

type EntityError struct {
	Level  Level  `json:"level"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

type Level string

const (
	LevelInfo    = "info"
	LevelWarning = "warning"
	LevelError   = "error"
)
