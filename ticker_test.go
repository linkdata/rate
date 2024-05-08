package rate

import (
	"sync"
	"testing"
	"time"
)

func TestTickerClosing(t *testing.T) {
	var wantcounter int64
	ticker := NewTicker(nil, nil)
	ticker.Close()
	select {
	case _, ok := <-ticker.C:
		if ok {
			t.Error("got a tick")
		}
	default:
	}
	if counter := ticker.Count(); counter != wantcounter {
		t.Error("counter is", counter, ", but expected", wantcounter)
	}
}

func TestTickerClosingDrainsTicks(t *testing.T) {
	var wantcounter int64
	ticker := NewTicker(nil, nil)
	ticker.Close()

	// resurrect
	ticker.tickCh = make(chan struct{})
	ticker.closeCh = make(chan struct{})

	go func() {
		defer close(ticker.tickCh)
		var done bool
		for !done {
			ticker.tickCh <- struct{}{}
			ticker.mu.Lock()
			ticker.counter++
			done = ticker.counter > 10
			ticker.mu.Unlock()
		}
	}()

	ticker.Close()

	select {
	case _, ok := <-ticker.C:
		if ok {
			t.Error("got a tick")
		}
	default:
	}
	if counter := ticker.Count(); counter != wantcounter {
		t.Error("counter is", counter, ", but expected", wantcounter)
	}
}

func TestTickerClosingWithWaiters(t *testing.T) {
	maxrate := int32(time.Second / variance * 2)
	ticker := NewTicker(nil, &maxrate)
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
	now := time.Now()
	ticker := NewTicker(nil, nil)
	for i := 0; i < n; i++ {
		_, ok := <-ticker.C
		if !ok {
			t.Error("ticker channel closed early")
		}
	}
	for i := 0; i < 10; i++ {
		if ticker.Count() == n {
			break
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(time.Millisecond)
	if x := ticker.Count(); x != n {
		t.Errorf("%v != %v", x, n)
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}

func TestNewSubTicker(t *testing.T) {
	const n = 100
	now := time.Now()
	t1 := NewTicker(nil, nil)
	t2 := NewTicker(t1.C, nil)
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
		if t2.Count() == n {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if x := t2.Count(); x != n {
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
	var wantcounter int64

	maxrate := int32(100)
	ticker := NewTicker(nil, &maxrate)
	period := time.Second / time.Duration(maxrate)

	now := time.Now()

	// after one Wait, we should now be able to consume two ticks in one period
	ticker.Wait()
	<-ticker.C
	wantcounter++
	<-ticker.C
	wantcounter++

	elapsed := time.Since(now)
	if elapsed < period {
		t.Error("ticks came too fast", elapsed, period)
	}
	if elapsed > (period*12)/10 { // 20% margin
		t.Error("ticks came too slow", elapsed, period)
	}

	ticker.Close()
	if counter := ticker.Count(); counter != wantcounter {
		t.Error("counter is", counter, ", but expected", wantcounter)
	}
}

func TestWaitTwice(t *testing.T) {
	var wantcounter int64

	now := time.Now()
	maxrate := int32(time.Second / variance * 2)
	ticker := NewTicker(nil, &maxrate)

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
	if counter := ticker.Count(); counter != wantcounter {
		t.Error("counter is", counter, ", but expected", wantcounter)
	}
}

func TestWaitFullRate(t *testing.T) {
	tickerTimerDuration = time.Millisecond
	defer func() {
		tickerTimerDuration = time.Second
	}()
	maxrate := int32(1000)
	n := int(variance / 2 / (time.Second / time.Duration(maxrate)))
	if n < 3 {
		panic(n)
	}
	ticker := NewTicker(nil, &maxrate)

	if load := ticker.Load(); load != 0 {
		t.Error("load out of spec", load)
	}

	now := time.Now()

	for i := 0; i < n; i++ {
		ticker.Wait()
		if rate := ticker.Rate(); rate < 1 || rate > maxrate {
			t.Error("rate out of spec", rate)
		}
		if load := ticker.Load(); load < 1 || load > 1000 {
			t.Error("load out of spec", load)
		}
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
	if rate := ticker.Rate(); rate < maxrate/2 {
		t.Error("rate out of spec", rate)
	}
	if load := ticker.Load(); load < 500 || load > 1000 {
		t.Error("load out of spec", load)
	}
}

func TestInitialLoad(t *testing.T) {
	maxrate := int32(100000)
	ticker := NewTicker(nil, &maxrate)
	if load := ticker.Load(); load != 0 {
		t.Error("load out of spec", load)
	}
	go func() {
		for range ticker.C {
		}
	}()

	for ticker.Count() < 1100 {
	}

	for i := 0; i < 1000; i++ {
		if load := ticker.Load(); load < 10 || load > 1000 {
			t.Error("load out of spec", load, ticker.Count(), ticker.Rate())
		}
	}
	ticker.Close()
}
