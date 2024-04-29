package rate

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestTickerRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := NewTicker(ctx, nil, nil)
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("got a tick")
		}
	default:
	}
}

func TestNewTicker(t *testing.T) {
	const n = 100
	var counter uint64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := NewTicker(ctx, nil, &counter)
	now := time.Now()
	for i := 0; i < n; i++ {
		_, ok := <-ch
		if !ok {
			t.Error("ticker channel closed early")
		}
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
	for i := 0; i < 10; i++ {
		if atomic.LoadUint64(&counter) == n {
			break
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(time.Millisecond)
	if x := atomic.LoadUint64(&counter); x != n {
		t.Errorf("%v != %v", x, n)
	}
}

func TestNewSubTicker(t *testing.T) {
	const n = 100
	var counter uint64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch1 := NewTicker(ctx, nil, nil)
	ch2 := NewSubTicker(ch1, nil, &counter)
	now := time.Now()
	for i := 0; i < n; i++ {
		_, ok := <-ch2
		if !ok {
			t.Error("ticker channel closed early")
		}
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
	for i := 0; i < 10; i++ {
		if atomic.LoadUint64(&counter) == n {
			break
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(time.Millisecond)
	if x := atomic.LoadUint64(&counter); x != n {
		t.Errorf("%v != %v", x, n)
	}
}
