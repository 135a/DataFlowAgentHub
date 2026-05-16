package ssebus

import (
	"testing"
)

func TestBusDropCounter(t *testing.T) {
	b := New()

	// Initial value should be 0
	if b.TotalDrops() != 0 {
		t.Errorf("initial TotalDrops() = %d, want 0", b.TotalDrops())
	}

	// Subscribe and publish normally
	ch := b.Subscribe("test-session")
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

	b.Unsubscribe("test-session", ch)
}

func TestBusDropCounterIncrement(t *testing.T) {
	b := New()
	ch := b.Subscribe("test-session")

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

	b.Unsubscribe("test-session", ch)
}
