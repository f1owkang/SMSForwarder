package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

type Logger struct {
	mu      sync.Mutex
	console io.Writer
	file    io.Writer
	now     func() time.Time
}

func New(console, file io.Writer) *Logger {
	return &Logger{console: console, file: file, now: time.Now}
}

func (l *Logger) log(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := l.now().Format("2006-01-02 15:04:05")
	if l.file != nil {
		if b, err := json.Marshal(map[string]string{
			"type": "log", "timestamp": ts, "level": level, "message": msg,
		}); err == nil {
			fmt.Fprintf(l.file, "%s\n", b)
		}
	}
	fmt.Fprintf(l.console, "[%s] %s\n", level, msg)
}

func (l *Logger) Info(msg string)  { l.log("INFO", msg) }
func (l *Logger) Warn(msg string)  { l.log("WARNING", msg) }
func (l *Logger) Error(msg string) { l.log("ERROR", msg) }

func (l *Logger) Record(fields map[string]any, summary string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		rec := map[string]any{"type": "record"}
		for k, v := range fields {
			rec[k] = v
		}
		if b, err := json.Marshal(rec); err == nil {
			fmt.Fprintf(l.file, "%s\n", b)
		}
	}
	fmt.Fprintf(l.console, "[INFO] %s\n", summary)
}
