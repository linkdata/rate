package rate

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTickerClosing(t *testing.T) {
	maxrate := int32(time.Second / variance * 2)
	ticker := NewTicker(&maxrate, nil)
	ticker.Close()
	select {
	case _, ok := <-ticker.C:
		if ok {
			t.Error("got a tick")
		}
	default:
	}
}

func TestNewTicker(t *testing.T) {
	const n = 100
	var counter uint64
	now := time.Now()
	ticker := NewTicker(nil, &counter)
	for i := 0; i < n; i++ {
		_, ok := <-ticker.C
		if !ok {
			t.Error("ticker channel closed early")
		}
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
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}

func TestNewSubTicker(t *testing.T) {
	const n = 100
	var counter uint64
	now := time.Now()
	t1 := NewTicker(nil, nil)
	t2 := NewSubTicker(t1.C, nil, &counter)
	for i := 0; i < n; i++ {
		_, ok := <-t2.C
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
	if x := atomic.LoadUint64(&counter); x != n {
		t.Errorf("%v != %v", x, n)
	}
	t1.Close()
	// there can be at most one extra tick to read after t1.Close
	if _, ok := <-t2.C; ok {
		if _, ok := <-t2.C; ok {
			t.Error("t2 should have been closed")
		}
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}

func TestAddingTick(t *testing.T) {
	var counter uint64

	now := time.Now()
	maxrate := int32(time.Second / variance * 2)
	ticker := NewTicker(&maxrate, &counter)

	select {
	case <-ticker.C:
	case <-time.NewTimer(variance).C:
		t.Error("timed out waiting for tick")
	}

	select {
	case <-ticker.C:
		t.Error("got an unexpected tick")
	default:
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ticker.C:
		case <-time.NewTimer(variance).C:
			t.Error("timed out waiting for tick")
		}
	}()
	ticker.AddTick(time.Nanosecond)
	wg.Wait()
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
	ticker.Close()
	if counter != 1 {
		t.Error("counter should be one, not", counter)
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}
