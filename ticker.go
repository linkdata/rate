package rate

import (
	"sync"
	"sync/atomic"
	"time"
)

type Ticker struct {
	C           <-chan struct{} // Sends a struct{}{} at most maxrate times per second
	WorkerMax   int32           // Maximum number of workers that may be started with Worker(), default 10000
	WorkerLoad  int32           // Load at which we stop starting new workers, default 1000
	WorkerRatio int32           // Ratio of max workers to max rate, default 1
	tickCh      chan struct{}   // source for C, closed by runner
	parent      *Ticker         // parent Ticker, or nil
	maxrate     *int32          // (atomic) maxrate pointer, or nil
	workers     int32           // (atomic) number of workers started by Worker()
	mu          sync.Mutex      // protects following
	closeCh     chan struct{}   // channel signalling Close() is called
	counter     int64           // counter
	padding     int32           // padding added by Wait
	rate        int32           // current rate
	load        int32           // current load in permille
}

// TickerTimerInterval is how often a Ticker updates it's rate and load metrics.
var TickerTimerInterval = time.Second

// Close stops the Ticker and frees resources.
//
// It is safe to call multiple times or concurrently.
// Once Close() returns, no more ticks will be delivered, and if you passed a
// non-nil ticker counter to NewTicker(), it will be correct.
func (ticker *Ticker) Close() {
	ticker.mu.Lock()
	if ticker.closeCh != nil {
		close(ticker.closeCh)
		ticker.closeCh = nil
	}
	ticker.mu.Unlock()
	drained := ticker.Drain()
	ticker.mu.Lock()
	ticker.counter -= drained
	ticker.counter -= int64(ticker.padding)
	ticker.mu.Unlock()
}

// IsClosed returns true if the Ticker is closed.
func (ticker *Ticker) IsClosed() (yes bool) {
	ticker.mu.Lock()
	yes = ticker.closeCh == nil
	ticker.mu.Unlock()
	return
}

// Drain consumes ticks from the Ticker until it is closed. Returns the number of ticks drained.
func (ticker *Ticker) Drain() (drained int64) {
	for range ticker.tickCh {
		drained++
	}
	return
}

func (ticker *Ticker) maxWorkers() (n int32) {
	n = ticker.MaxRate() * ticker.WorkerRatio
	if n < 1 || n > ticker.WorkerMax {
		n = ticker.WorkerMax
	}
	return
}

// Worker starts f when the current load allows.
// Returns true if the goroutine was started.
// Returns false if the ticker is no longer running.
//
// It limits the number of goroutines started this way to MaxRate() * WorkerRatio,
// with a hard cap of WorkerMax. If MaxRate() * WorkerRatio is less than one,
// WorkerMax is used.
func (ticker *Ticker) Worker(f func()) (ok bool) {
	if ok = ticker.Load() < ticker.WorkerLoad || ticker.Wait(); ok {
		var sleepTime time.Duration
		for ok && atomic.LoadInt32(&ticker.workers) > ticker.maxWorkers() {
			if ok = !ticker.IsClosed(); ok {
				if sleepTime < time.Millisecond*100 {
					sleepTime += time.Second / SleepGranularity
				}
				time.Sleep(sleepTime)
			}
		}
		if ok {
			atomic.AddInt32(&ticker.workers, 1)
			go func() {
				defer atomic.AddInt32(&ticker.workers, -1)
				f()
			}()
		}
	}
	return
}

// Wait delays until the next tick is available, then adds a "free tick" back to the Ticker.
//
// Returns true if we waited successfully, or false if the Ticker is closed.
func (ticker *Ticker) Wait() (ok bool) {
	if _, ok = <-ticker.tickCh; ok {
		for ticker.parent != nil {
			ticker = ticker.parent
		}
		ticker.mu.Lock()
		ticker.padding++
		ticker.mu.Unlock()
	}
	return
}

// Count returns the number of ticks delivered so far.
func (ticker *Ticker) Count() (n int64) {
	ticker.mu.Lock()
	n = ticker.counter
	ticker.mu.Unlock()
	return
}

// WorkerCount returns the number of currently running workers.
func (ticker *Ticker) WorkerCount() int32 {
	return atomic.LoadInt32(&ticker.workers)
}

// Rate returns the current rate of ticks per second.
func (ticker *Ticker) Rate() (n int32) {
	ticker.mu.Lock()
	n = ticker.rate
	ticker.mu.Unlock()
	return
}

// MaxRate returns the current max rate of ticks per second.
func (ticker *Ticker) MaxRate() (n int32) {
	if ticker.maxrate != nil {
		n = atomic.LoadInt32(ticker.maxrate)
	}
	return
}

// Load returns the current load in permille.
//
// Load is rounded up, and is only zero if the rate is zero.
// If the Ticker has parent Ticker(s), the highest load is returned.
// Closed Tickers have a load of 1000.
func (ticker *Ticker) Load() (load int32) {
	for ticker != nil {
		ticker.mu.Lock()
		if ticker.load > load {
			load = ticker.load
		}
		ticker.mu.Unlock()
		ticker = ticker.parent
	}
	return
}

// LoadForRate calculates the load given a rate and maxrate.
func LoadForRate(rate int32, maxrate *int32) (load int32) {
	if maxrate != nil {
		if mr := atomic.LoadInt32(maxrate); mr > 0 {
			mr *= 10
			rate *= 10
			if mr > 10000 {
				// always round up the load
				rate += (mr / 1000) - 1
			}
			if load = (rate * 1000) / mr; load > 1000 {
				load = 1000
			}
		}
	}
	return
}

func (ticker *Ticker) run(closeCh <-chan struct{}, parent *Ticker) {
	timer := time.NewTimer(TickerTimerInterval)
	defer func() {
		close(ticker.tickCh)
		timer.Stop()
		ticker.mu.Lock()
		ticker.load = 1000
		ticker.rate = 0
		ticker.mu.Unlock()
	}()

	var rl Limiter
	var tickCh chan struct{}
	var parentCh <-chan struct{}

	rateWhen := time.Now()
	rateCount := ticker.counter
	needElapsed := (TickerTimerInterval * 10) / 11
	if parent != nil {
		parentCh = parent.C
	} else {
		tickCh = ticker.tickCh
	}

	for !ticker.IsClosed() {
		select {
		case tickCh <- struct{}{}:
			// sent a tick to a consumer
			if parent != nil {
				parentCh = parent.C
				tickCh = nil
			}
			ticker.mu.Lock()
			doWait := ticker.padding == 0
			if doWait {
				ticker.counter++
				if rateCount == 0 {
					// emulate some load before first actual measurement
					ticker.rate++
					ticker.load = LoadForRate(ticker.rate, ticker.maxrate)
				}
			} else {
				ticker.padding--
			}
			ticker.mu.Unlock()
			if doWait {
				rl.Wait(ticker.maxrate)
			}
		case _, ok := <-parentCh:
			// if parentCh is not nil, we require a successful read from it
			if !ok {
				return
			}
			parentCh = nil
			tickCh = ticker.tickCh
		case <-timer.C:
			// update current rate and load
			ticker.mu.Lock()
			if delta := ticker.counter - rateCount; delta > 0 {
				if elapsed := time.Since(rateWhen); elapsed >= needElapsed {
					rateWhen = rateWhen.Add(elapsed)
					rateCount = ticker.counter
					ticker.rate = int32(time.Duration(delta) * time.Second / elapsed)
					ticker.load = LoadForRate(ticker.rate, ticker.maxrate)

				}
			}
			ticker.mu.Unlock()
		case <-closeCh:
			return
		}
	}
}

// NewTicker returns a Ticker that reads ticks from a parent Ticker
// and sends a `struct{}{}` at most `*maxrate` times per second.
//
// The effective max rate is thus the lower of the parent Tickers
// maxrate and this Tickers `*maxrate`.
//
// A nil `parent` Ticker means tick rate is only limited by `maxrate`.
// If the parent Ticker is closed, this Ticker will stop sending ticks.
//
// A nil `maxrate` or a `*maxrate` of zero or less sends
// as quickly as possible, so only limited by the parent channel.
func NewTicker(parent *Ticker, maxrate *int32) *Ticker {
	if parent != nil {
		if maxrate == nil {
			maxrate = parent.maxrate
		}
	}
	ticker := &Ticker{
		WorkerMax:   10000,
		WorkerLoad:  1000,
		WorkerRatio: 1,
		tickCh:      make(chan struct{}),
		closeCh:     make(chan struct{}),
		parent:      parent,
		maxrate:     maxrate,
	}
	ticker.C = ticker.tickCh
	go ticker.run(ticker.closeCh, parent)
	return ticker
}
