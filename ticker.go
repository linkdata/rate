package rate

import (
	"sync"
	"sync/atomic"
	"time"
)

type Ticker struct {
	C       <-chan struct{} // sends a struct{}{} at most maxrate times per second
	tickCh  chan struct{}   // source for C, closed by runner
	maxrate *int32          // maxrate pointer, or nil
	mu      sync.Mutex      // protects following
	closeCh chan struct{}   // channel signalling Close() is called
	counter int64           // counter
	rate    int32           // current rate
	load    int32           // current load in permille
	padding int32           // padding added by Wait
}

var tickerTimerDuration = time.Second

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
	// wait for tick channel to close
	var drained int64
	for range ticker.tickCh {
		drained++
	}
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

// Wait delays until the next tick is available, then adds a "free tick" back to the Ticker.
//
// Returns true if we waited successfully, or false if the Ticker is closed.
//
// Typical use case is to launch goroutines that in turn uses the Ticker to rate limit some resource or action,
// thus limiting the rate of goroutines spawning without impacting the resource use rate.
func (ticker *Ticker) Wait() (ok bool) {
	if _, ok = <-ticker.tickCh; ok {
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

// Rate returns the current rate of ticks per second.
func (ticker *Ticker) Rate() (n int32) {
	ticker.mu.Lock()
	n = ticker.rate
	ticker.mu.Unlock()
	return
}

// Load returns the current load in permille, or -1 if the rate is unlimited.
//
// Load is rounded up, and is only zero if the rate is zero.
func (ticker *Ticker) Load() (n int32) {
	ticker.mu.Lock()
	n = ticker.load
	ticker.mu.Unlock()
	return
}

func (ticker *Ticker) calcLoadLocked() {
	load := int32(-1)
	if ticker.maxrate != nil {
		if mr := atomic.LoadInt32(ticker.maxrate); mr > 0 {
			mr *= 10
			rate := ticker.rate * 10
			if mr > 10000 {
				// always round up the load
				rate += (mr / 1000) - 1
			}
			load = (rate * 1000) / mr
		}
	}
	ticker.load = load
}

func (ticker *Ticker) run(closeCh <-chan struct{}, parent *Ticker) {
	timer := time.NewTimer(tickerTimerDuration)
	defer func() {
		close(ticker.tickCh)
		timer.Stop()
	}()

	var rl Limiter
	var tickCh chan struct{}
	var parentCh <-chan struct{}

	rateWhen := time.Now()
	rateCount := ticker.counter
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
					ticker.calcLoadLocked()
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
				rateCount = ticker.counter
				elapsed := time.Since(rateWhen)
				rateWhen = rateWhen.Add(elapsed)
				ticker.rate = int32(time.Duration(delta) * time.Second / elapsed)
				ticker.calcLoadLocked()
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
	if maxrate == nil && parent != nil {
		maxrate = parent.maxrate
	}
	ticker := &Ticker{
		tickCh:  make(chan struct{}),
		closeCh: make(chan struct{}),
		maxrate: maxrate,
	}
	ticker.C = ticker.tickCh
	go ticker.run(ticker.closeCh, parent)
	return ticker
}
