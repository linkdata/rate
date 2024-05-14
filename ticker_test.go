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
	defer ticker.Close()
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
	defer t1.Close()
	t2 := NewTicker(t1, nil)
	defer t2.Close()
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
	defer ticker.Close()

	if ticker.MaxRate() != maxrate {
		t.Fatal("incorrect maxrate")
	}

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
	defer ticker.Close()

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

/*func TestWaitFullRate_100(t *testing.T) {
	for i := 0; i < 100; i++ {
		TestWaitFullRate(t)
	}
}*/

func TestWaitFullRate(t *testing.T) {
	tickerTimerDuration = time.Second / 10
	defer func() {
		tickerTimerDuration = time.Second
	}()
	maxrate := int32(1000)
	parent := NewTicker(nil, &maxrate)
	defer parent.Close()
	ticker := NewTicker(parent, nil)
	defer ticker.Close()

	if load := ticker.Load(); load != 0 {
		t.Error("load out of spec", load)
	}

	// allow 1% over maxrate to account for sub-second tickerTimerDuration
	maxratelimit := (maxrate * 101) / 100
	now := time.Now()

	for time.Since(now) < tickerTimerDuration*2 {
		ticker.Wait()
		if rate := ticker.Rate(); rate < 1 || rate > maxratelimit {
			t.Fatal("rate out of spec", rate, ticker.Count())
		}
		if load := ticker.Load(); load < 1 || load > 1000 {
			t.Fatal("load out of spec", load)
		}
		_, ok := <-ticker.C
		if !ok {
			t.Fatal("ticker channel closed early")
		}
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

func TestTicker_calcLoadLocked(t *testing.T) {
	tests := []struct {
		name    string
		maxrate int32
		rate    int32
		load    int32
	}{
		{"unlimited", 0, 0, 0},
		{"1000,0", 1000, 0, 0},
		{"1000,1", 1000, 1, 1},
		{"1000,1000", 1000, 1000, 1000},
		{"1000,1001", 1000, 1001, 1000},
		{"100,1", 100, 1, 10},
		{"1500,1", 1500, 1, 1},
		{"1500,1499", 1500, 1499, 1000},
		{"2000,1", 2000, 1, 1},
		{"2000,2", 2000, 2, 1},
		{"2000,3", 2000, 3, 2},
		{"2000,1999", 2000, 1999, 1000},
		{"10000,1", 10000, 1, 1},
		{"10000,9990", 10000, 9990, 999},
		{"10000,9991", 10000, 9991, 1000},
		{"10000,9999", 10000, 9999, 1000},
		{"10000,10000", 10000, 10000, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker := &Ticker{
				maxrate: &tt.maxrate,
				rate:    tt.rate,
			}
			ticker.calcLoadLocked()
			if ticker.load != tt.load {
				t.Error("ticker.load is", ticker.load, "wanted", tt.load)
			}
		})
	}
}

func TestWorkerUnlimited(t *testing.T) {
	maxrate := int32(MaxWorkers)
	ticker := NewTicker(nil, &maxrate)
	defer ticker.Close()
	var wg sync.WaitGroup

	now := time.Now()
	wg.Add(1)
	ticker.Worker(2, func() { defer wg.Done() }) // overflows MaxWorkers
	for time.Since(now) < (variance*8)/10 {
		wg.Add(1)
		ticker.Worker(0, func() { defer wg.Done() }) // zero mult means use MaxWorkers
	}
	wg.Wait()
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}

func TestWorkerLimited(t *testing.T) {
	maxrate := int32(100)
	ticker := NewTicker(nil, &maxrate)
	defer ticker.Close()
	var wg sync.WaitGroup

	now := time.Now()
	var calls int32
	for time.Since(now) < variance/2 {
		wg.Add(1)
		calls++
		ticker.Worker(2, func() {
			defer wg.Done()
			time.Sleep(variance / 2)
		})
	}
	wg.Wait()
	wantElapsed := (variance / 2) * time.Duration(calls-(maxrate*2))
	if d := time.Since(now); d < wantElapsed {
		t.Errorf("%v < %v", d, wantElapsed)
	}
	if d := time.Since(now); d > wantElapsed*2 {
		t.Errorf("%v > %v", d, wantElapsed*2)
	}
}
