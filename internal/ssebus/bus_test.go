package ssebus

import (
	"testing"

	"go.uber.org/zap"
)

func newTestMemoryBus() *MemoryBus {
	return NewMemoryBus(zap.NewNop())
}

func TestBusDropCounter(t *testing.T) {
	b := newTestMemoryBus()

	// Initial value should be 0
	if b.TotalDrops() != 0 {
		t.Errorf("initial TotalDrops() = %d, want 0", b.TotalDrops())
	}

	// Subscribe and publish normally
	ch, cancel := b.Subscribe("test-session")
	defer cancel()
	b.Publish("test-session", Event{Type: "test", Data: "hello"})

	// Should receive the event
	select {
	case ev := <-ch:
		if ev.Type != "test" {
			t.Errorf("expected type 'test', got '%s'", ev.Type)
		}
	default:
		t.Error("expected to receive event, got nothing")
	}

	// No drops should have occurred
	if b.TotalDrops() != 0 {
		t.Errorf("after normal publish, TotalDrops() = %d, want 0", b.TotalDrops())
	}
}

func TestBusDropCounterIncrement(t *testing.T) {
	b := newTestMemoryBus()
	ch, cancel := b.Subscribe("test-session")
	defer cancel()

	// Fill the buffer (capacity 32) without consuming
	for i := 0; i < 32; i++ {
		b.Publish("test-session", Event{Type: "fill", Data: i})
	}

	// Buffer is now full — next publishes should drop
	b.Publish("test-session", Event{Type: "drop", Data: "should_drop"})
	if b.TotalDrops() != 1 {
		t.Errorf("after 1st overflow, TotalDrops() = %d, want 1", b.TotalDrops())
	}

	// Publish more drops
	for i := 0; i < 5; i++ {
		b.Publish("test-session", Event{Type: "drop", Data: i})
	}
	if b.TotalDrops() != 6 {
		t.Errorf("after 6 overflows, TotalDrops() = %d, want 6", b.TotalDrops())
	}

	// Drain the buffer and verify full events are intact
	for i := 0; i < 32; i++ {
		select {
		case ev := <-ch:
			if ev.Type != "fill" {
				t.Errorf("event %d: expected type 'fill', got '%s'", i, ev.Type)
			}
		default:
			t.Errorf("expected buffered event %d, got nothing", i)
		}
	}

	// Buffer is drained; cancel should work without panic
	cancel()
}

func TestBusInterface(t *testing.T) {
	// Verify that MemoryBus satisfies the Bus interface
	var b Bus = newTestMemoryBus()
	_ = b

	// Basic interface contract: subscribe -> publish -> receive
	ch, cancel := b.Subscribe("iface-session")
	defer cancel()

	b.Publish("iface-session", Event{Type: "ping", Data: "pong"})

	select {
	case ev := <-ch:
		if ev.Type != "ping" {
			t.Errorf("expected type 'ping', got '%s'", ev.Type)
		}
	default:
		t.Error("expected to receive event via Bus interface")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b := newTestMemoryBus()

	ch1, cancel1 := b.Subscribe("multi-session")
	defer cancel1()
	ch2, cancel2 := b.Subscribe("multi-session")
	defer cancel2()

	b.Publish("multi-session", Event{Type: "broadcast", Data: "hello"})

	// Both subscribers should receive the event
	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Type != "broadcast" {
				t.Errorf("subscriber %d: expected type 'broadcast', got '%s'", i, ev.Type)
			}
		default:
			t.Errorf("subscriber %d: expected to receive event", i)
		}
	}
}
