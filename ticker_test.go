package rate

import (
	"context"
	"testing"
	"time"
)

func TestTickerRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := NewTicker(ctx, nil)
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("got a tick")
		}
	default:
	}
}

func TestNewTicker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := NewTicker(ctx, nil)
	now := time.Now()
	for i := 0; i < 100; i++ {
		_, ok := <-ch
		if !ok {
			t.Error("ticker channel closed early")
		}
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}

func TestNewSubTicker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch1 := NewTicker(ctx, nil)
	ch2 := NewSubTicker(ch1, nil)
	now := time.Now()
	for i := 0; i < 100; i++ {
		_, ok := <-ch2
		if !ok {
			t.Error("ticker channel closed early")
		}
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}
