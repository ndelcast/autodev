package web

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// LogEntry represents a single log line.
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Attrs   string
}

// LogBuffer stores recent log entries in a ring buffer and notifies subscribers.
type LogBuffer struct {
	mu          sync.RWMutex
	entries     []LogEntry
	maxEntries  int
	subscribers map[chan LogEntry]struct{}
}

// NewLogBuffer creates a buffer that keeps the last maxEntries logs.
func NewLogBuffer(maxEntries int) *LogBuffer {
	return &LogBuffer{
		entries:     make([]LogEntry, 0, maxEntries),
		maxEntries:  maxEntries,
		subscribers: make(map[chan LogEntry]struct{}),
	}
}

// Add appends a log entry and notifies subscribers.
func (lb *LogBuffer) Add(entry LogEntry) {
	lb.mu.Lock()
	if len(lb.entries) >= lb.maxEntries {
		lb.entries = lb.entries[1:]
	}
	lb.entries = append(lb.entries, entry)

	// Notify subscribers (non-blocking)
	for ch := range lb.subscribers {
		select {
		case ch <- entry:
		default:
		}
	}
	lb.mu.Unlock()
}

// Entries returns a copy of all stored entries.
func (lb *LogBuffer) Entries() []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	result := make([]LogEntry, len(lb.entries))
	copy(result, lb.entries)
	return result
}

// Subscribe returns a channel that receives new log entries.
// Call Unsubscribe when done.
func (lb *LogBuffer) Subscribe() chan LogEntry {
	ch := make(chan LogEntry, 64)
	lb.mu.Lock()
	lb.subscribers[ch] = struct{}{}
	lb.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (lb *LogBuffer) Unsubscribe(ch chan LogEntry) {
	lb.mu.Lock()
	delete(lb.subscribers, ch)
	lb.mu.Unlock()
	close(ch)
}

// SlogHandler is a slog.Handler that writes to a LogBuffer
// and delegates to a wrapped handler for normal output.
type SlogHandler struct {
	buffer  *LogBuffer
	wrapped slog.Handler
	attrs   []slog.Attr
	groups  []string
}

// NewSlogHandler creates a handler that logs to both the buffer and a wrapped handler.
func NewSlogHandler(buffer *LogBuffer, wrapped slog.Handler) *SlogHandler {
	return &SlogHandler{
		buffer:  buffer,
		wrapped: wrapped,
	}
}

func (h *SlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.wrapped.Enabled(ctx, level)
}

func (h *SlogHandler) Handle(ctx context.Context, r slog.Record) error {
	// Build attrs string
	var attrs string
	r.Attrs(func(a slog.Attr) bool {
		if attrs != "" {
			attrs += " "
		}
		attrs += fmt.Sprintf("%s=%v", a.Key, a.Value)
		return true
	})
	// Include handler-level attrs
	for _, a := range h.attrs {
		if attrs != "" {
			attrs += " "
		}
		attrs += fmt.Sprintf("%s=%v", a.Key, a.Value)
	}

	h.buffer.Add(LogEntry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   attrs,
	})

	return h.wrapped.Handle(ctx, r)
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SlogHandler{
		buffer:  h.buffer,
		wrapped: h.wrapped.WithAttrs(attrs),
		attrs:   append(h.attrs, attrs...),
		groups:  h.groups,
	}
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	return &SlogHandler{
		buffer:  h.buffer,
		wrapped: h.wrapped.WithGroup(name),
		attrs:   h.attrs,
		groups:  append(h.groups, name),
	}
}
