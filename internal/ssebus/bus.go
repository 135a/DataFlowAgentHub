package ssebus

import (
	"sync"
)

// Event is a small JSON-serializable SSE payload.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Bus struct {
	mu   sync.Mutex
	subs map[string][]chan Event
}

func New() *Bus {
	return &Bus{subs: make(map[string][]chan Event)}
}

func (b *Bus) Subscribe(sessionID string) chan Event {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.subs[sessionID] = append(b.subs[sessionID], ch)
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(sessionID string, ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.subs[sessionID]
	out := list[:0]
	for _, c := range list {
		if c != ch {
			out = append(out, c)
		} else {
			close(c)
		}
	}
	if len(out) == 0 {
		delete(b.subs, sessionID)
	} else {
		b.subs[sessionID] = out
	}
}

func (b *Bus) Publish(sessionID string, ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[sessionID] {
		select {
		case ch <- ev:
		default:
		}
	}
}
