package ssebus

import (
	"log/slog"
	"sync"
	"sync/atomic"
)

// Event 是一个小型可 JSON 序列化的 SSE 负载
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Bus struct {
	mu         sync.Mutex
	subs       map[string][]chan Event
	totalDrops atomic.Int64
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

func (b *Bus) TotalDrops() int64 { return b.totalDrops.Load() }

func (b *Bus) Publish(sessionID string, ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[sessionID] {
		select {
		case ch <- ev:
		default:
			drops := b.totalDrops.Add(1)
			if drops%100 == 0 {
				slog.Warn("sse event dropped", "session_id", sessionID, "total_drops", drops)
			}
		}
	}
}
