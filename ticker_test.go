package rate_test

import (
	"sync"
	"testing"
	"time"

	"github.com/linkdata/rate"
)

func TestTickerClosing(t *testing.T) {
	var wantcounter int64
	ticker := rate.NewTicker(nil, nil)
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

func TestTickerDrain(t *testing.T) {
	var drained int64
	n := 1

	for drained == 0 && n < 10 {
		var wg sync.WaitGroup
		ticker := rate.NewTicker(nil, nil)
		wg.Add(1)
		go func() {
			defer wg.Done()
			drained = ticker.Drain()
		}()
		time.Sleep(time.Millisecond * time.Duration(n))
		ticker.Close()
		wg.Wait()
		n++
	}
	if drained == 0 {
		t.Error("failed to drain ticks")
	}
}

func TestTickerClosingWithWaiters(t *testing.T) {
	maxrate := int32(time.Second / variance * 2)
	ticker := rate.NewTicker(nil, &maxrate)
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

func TestTickerClosingIsIdempotentAfterWait(t *testing.T) {
	maxrate := int32(100000)
	ticker := rate.NewTicker(nil, &maxrate)

	if ok := ticker.Wait(); !ok {
		t.Fatal("ticker closed early")
	}

	ticker.Close()
	firstCount := ticker.Count()

	ticker.Close()
	secondCount := ticker.Count()

	if secondCount != firstCount {
		t.Fatalf("counter changed after repeated Close: first=%d second=%d", firstCount, secondCount)
	}
}

func TestNewTicker(t *testing.T) {
	const n = 100
	now := time.Now()
	ticker := rate.NewTicker(nil, nil)
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
	t1 := rate.NewTicker(nil, nil)
	defer t1.Close()
	t2 := rate.NewTicker(t1, nil)
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
	ticker := rate.NewTicker(nil, &maxrate)
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
	ticker := rate.NewTicker(nil, &maxrate)
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

func TestWaitFullRate(t *testing.T) {
	rate.TickerTimerInterval = time.Second / 10
	defer func() {
		rate.TickerTimerInterval = time.Second
	}()
	maxrate := int32(1000)
	parent := rate.NewTicker(nil, &maxrate)
	defer parent.Close()
	ticker := rate.NewTicker(parent, nil)
	defer ticker.Close()

	if load := ticker.Load(); load != 0 {
		t.Error("load out of spec", load)
	}

	// allow 1% over maxrate to account for sub-second tickerTimerDuration
	maxratelimit := (maxrate * 101) / 100
	now := time.Now()

	for time.Since(now) < rate.TickerTimerInterval*2 {
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

func TestTickerRateTracksRateChanges(t *testing.T) {
	rate.TickerTimerInterval = time.Millisecond * 40
	defer func() {
		rate.TickerTimerInterval = time.Second
	}()

	maxrate := int32(1500)
	ticker := rate.NewTicker(nil, &maxrate)
	defer ticker.Close()

	now := time.Now()
	for time.Since(now) < rate.TickerTimerInterval*3 {
		_, ok := <-ticker.C
		if !ok {
			t.Fatal("ticker channel closed early")
		}
	}

	highRate := ticker.Rate()
	if highRate < 500 {
		t.Fatal("failed to observe high rate", highRate)
	}

	maxrate = 100
	now = time.Now()
	for time.Since(now) < rate.TickerTimerInterval*6 {
		_, ok := <-ticker.C
		if !ok {
			t.Fatal("ticker channel closed early")
		}
	}

	lowRate := ticker.Rate()
	if lowRate > 300 {
		t.Fatalf("rate did not track maxrate change, high=%d low=%d", highRate, lowRate)
	}
}

func TestTickerRateDropsToZeroWhenIdle(t *testing.T) {
	rate.TickerTimerInterval = time.Millisecond * 40
	defer func() {
		rate.TickerTimerInterval = time.Second
	}()

	maxrate := int32(1000)
	ticker := rate.NewTicker(nil, &maxrate)
	defer ticker.Close()

	now := time.Now()
	for time.Since(now) < rate.TickerTimerInterval*3 {
		_, ok := <-ticker.C
		if !ok {
			t.Fatal("ticker channel closed early")
		}
	}

	if highRate := ticker.Rate(); highRate < 200 {
		t.Fatal("failed to observe non-idle rate", highRate)
	}

	time.Sleep(rate.TickerTimerInterval * 4)

	if got := ticker.Rate(); got != 0 {
		t.Fatalf("expected idle rate to reach zero, got %d", got)
	}
	if got := ticker.Load(); got != 0 {
		t.Fatalf("expected idle load to reach zero, got %d", got)
	}
}

func TestInitialLoad(t *testing.T) {
	maxrate := int32(100000)
	ticker := rate.NewTicker(nil, &maxrate)
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

func TestTicker_LoadForRate(t *testing.T) {
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
			load := rate.LoadForRate(tt.rate, &tt.maxrate)
			if load != tt.load {
				t.Error("load is", load, "wanted", tt.load)
			}
		})
	}
}

func TestTicker_LoadForRateLargeValues(t *testing.T) {
	maxrate := int32(3000000)
	load := rate.LoadForRate(maxrate/2, &maxrate)
	if load != 500 {
		t.Fatalf("load is %d, wanted 500", load)
	}
}

func TestTicker_LoadForRateNegativeRateClamped(t *testing.T) {
	maxrate := int32(100)
	load := rate.LoadForRate(-1, &maxrate)
	if load != 0 {
		t.Fatalf("load is %d, wanted 0", load)
	}
}

func TestWorkerHonorsMaximumConcurrentWorkers(t *testing.T) {
	maxrate := int32(1)
	ticker := rate.NewTicker(nil, &maxrate)
	defer ticker.Close()

	ticker.WorkerRatio = 1
	ticker.WorkerMax = 10
	ticker.WorkerLoad = 1000

	firstStarted := make(chan struct{})
	blockFirst := make(chan struct{})
	if ok := ticker.Worker(func() {
		close(firstStarted)
		<-blockFirst
	}); !ok {
		t.Fatal("failed to start first worker")
	}

	select {
	case <-firstStarted:
	case <-time.After(variance):
		t.Fatal("first worker did not start in time")
	}

	secondStarted := make(chan struct{})
	secondDone := make(chan bool, 1)
	go func() {
		secondDone <- ticker.Worker(func() {
			close(secondStarted)
		})
	}()

	time.Sleep(time.Millisecond * 2)

	select {
	case <-secondStarted:
		t.Fatal("second worker started while max worker count was already reached")
	default:
	}

	if workers := ticker.WorkerCount(); workers != 1 {
		t.Fatalf("worker count is %d, wanted 1", workers)
	}

	close(blockFirst)

	select {
	case ok := <-secondDone:
		if !ok {
			t.Fatal("second Worker call unexpectedly failed")
		}
	case <-time.After(variance * 4):
		t.Fatal("second Worker call did not complete in time")
	}
}

func TestWorkerTreatsNonPositiveWorkerMaxAsOne(t *testing.T) {
	maxrate := int32(1000)
	ticker := rate.NewTicker(nil, &maxrate)
	defer ticker.Close()

	ticker.WorkerRatio = 1
	ticker.WorkerLoad = 1000

	tests := []int32{0, -1}
	for _, workerMax := range tests {
		ticker.WorkerMax = workerMax

		started := make(chan struct{})
		done := make(chan bool, 1)
		go func() {
			done <- ticker.Worker(func() {
				close(started)
			})
		}()

		select {
		case ok := <-done:
			if !ok {
				t.Fatalf("Worker returned false for WorkerMax=%d", workerMax)
			}
		case <-time.After(variance * 4):
			t.Fatalf("Worker blocked for WorkerMax=%d", workerMax)
		}

		select {
		case <-started:
		case <-time.After(variance * 4):
			t.Fatalf("worker function did not start for WorkerMax=%d", workerMax)
		}
	}
}

func TestWorkerUnlimited(t *testing.T) {
	var wg sync.WaitGroup
	var maxrate int32

	ticker := rate.NewTicker(nil, &maxrate)
	defer ticker.Close()

	maxrate = ticker.WorkerMax

	wg.Add(1)
	ticker.WorkerRatio = 2 // overflows WorkerMax
	ticker.Worker(func() { defer wg.Done() })
	ticker.WorkerRatio = 0 // zero ratio means use WorkerMax
	for i := int32(0); i < ticker.WorkerMax; i++ {
		wg.Add(1)
		ticker.Worker(func() { defer wg.Done() })
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(variance * 20):
		t.Fatal("workers did not complete in time")
	}

	deadline := time.Now().Add(variance * 10)
	for time.Now().Before(deadline) {
		if ticker.WorkerCount() == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if n := ticker.WorkerCount(); n != 0 {
		t.Fatalf("worker count is %d, wanted 0", n)
	}
}

func TestWorkerLimited(t *testing.T) {
	maxrate := int32(100)
	ticker := rate.NewTicker(nil, &maxrate)
	defer ticker.Close()
	ticker.WorkerRatio = 2
	var wg sync.WaitGroup

	now := time.Now()
	var calls int32
	for time.Since(now) < variance/2 {
		wg.Add(1)
		calls++
		ticker.Worker(func() {
			defer wg.Done()
			time.Sleep(variance / 2)
		})
	}
	wg.Wait()
	wantElapsed := (variance / 2) * time.Duration(calls-(maxrate*2))
	if d := time.Since(now); d < wantElapsed {
		t.Errorf("%v < %v", d, wantElapsed)
	}
	if d := time.Since(now); d > wantElapsed*2+variance {
		t.Errorf("%v > %v", d, wantElapsed*2+variance)
	}
}
