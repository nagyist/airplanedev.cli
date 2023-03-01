package dev_errors

type AppError struct {
	Level   Level  `json:"level"`
	AppName string `json:"name"`
	AppKind string `json:"kind"`
	Reason  string `json:"reason"`
}

type Level string

const (
	LevelInfo    = "info"
	LevelWarning = "warning"
	LevelError   = "error"
)
