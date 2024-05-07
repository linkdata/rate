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

func TestTickerClosingWithWaiters(t *testing.T) {
	maxrate := int32(time.Second / variance * 2)
	ticker := NewTicker(&maxrate, nil)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker.Wait()
		}()
	}
	ticker.Wait()
	ticker.Close()
	wg.Wait()
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

func TestWait(t *testing.T) {
	var counter, wantcounter uint64

	now := time.Now()
	maxrate := int32(time.Second / variance * 2)
	ticker := NewTicker(&maxrate, &counter)
	shouldwait := time.Second / time.Duration(maxrate)

	ticker.Wait()

	select {
	case <-ticker.C:
		wantcounter++
	case <-time.NewTimer(shouldwait).C:
		t.Error("tick not immediately available")
	}

	select {
	case <-ticker.C:
		wantcounter++
		t.Error("tick immediately available")
	default:
	}

	go ticker.Wait()
	ticker.Wait()

	if d := time.Since(now); d < shouldwait {
		t.Error("did not wait enough", d, shouldwait)
	}

	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
	ticker.Close()
	if counter != wantcounter {
		t.Error("counter is", counter, ", but expected", wantcounter)
	}
}

func TestWaitTwice(t *testing.T) {
	var counter, wantcounter uint64

	now := time.Now()
	maxrate := int32(time.Second / variance * 2)
	ticker := NewTicker(&maxrate, &counter)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		ticker.Wait()
	}()
	ticker.Wait()

	wg.Wait()

	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
	ticker.Close()
	if counter != wantcounter {
		t.Error("counter is", counter, ", but expected", wantcounter)
	}
}

func TestWaitFullRate(t *testing.T) {
	var counter uint64
	maxrate := int32(1000)
	n := int(variance / 2 / (time.Second / time.Duration(maxrate)))
	if n < 3 {
		panic(n)
	}
	ticker := NewTicker(&maxrate, &counter)
	now := time.Now()
	for i := 0; i < n; i++ {
		ticker.Wait()
		_, ok := <-ticker.C
		if !ok {
			t.Error("ticker channel closed early")
		}
	}
	expectsince := time.Second / time.Duration(maxrate) * time.Duration(n-1) // -1 since first tick is "free"
	if d := time.Since(now); d < expectsince {
		t.Errorf("%v < %v", d, expectsince)
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}
