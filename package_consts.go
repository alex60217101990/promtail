package promtail

type LogLevel int

const (
	Debug LogLevel = iota
	Info  LogLevel = iota
	Warn  LogLevel = iota
	Error LogLevel = iota
	// Maximum level, disables sending or printing
	Disable LogLevel = iota
)
