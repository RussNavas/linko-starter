package main


const LogContextKey contextKey = "log_context"
type LogContext struct {
	Username string
	Error error
}

