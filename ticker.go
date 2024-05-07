package rate

import (
	"sync"
	"sync/atomic"
	"time"
)

type Ticker struct {
	C       <-chan struct{} // sends a struct{}{} at most maxrate times per second
	tickCh  chan struct{}   // source for C, closed by runner
	waitCh  chan struct{}   // source for Wait, closed by runner
	mu      sync.Mutex      // protects closeCh
	closeCh chan struct{}   // channel signalling Close() is called
}

// Close stops the Ticker and frees resources.
//
// It is safe to call multiple times or concurrently.
func (ticker *Ticker) Close() {
	ticker.mu.Lock()
	defer ticker.mu.Unlock()
	if ticker.closeCh != nil {
		close(ticker.closeCh)
		ticker.closeCh = nil
	}
}

// Wait delays until the next tick is available.
func (ticker *Ticker) Wait() {
	<-ticker.waitCh
}

// Closed returns true if the Ticker is closed.
func (ticker *Ticker) Closed() (yes bool) {
	ticker.mu.Lock()
	yes = ticker.closeCh == nil
	ticker.mu.Unlock()
	return
}

func (ticker *Ticker) run(closeCh, parent <-chan struct{}, maxrate *int32, counter *uint64) {
	defer func() {
		close(ticker.waitCh)
		close(ticker.tickCh)
	}()
	var rl Limiter
	var timeCh <-chan time.Time
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	parentCh := parent
	tickCh := ticker.tickCh
	waitCh := ticker.waitCh

	if parent != nil {
		tickCh = nil
	}

	for !ticker.Closed() {
		select {
		case tickCh <- struct{}{}:
			// sent a tick to a consumer
			if counter != nil {
				atomic.AddUint64(counter, 1)
			}
			waitCh = ticker.waitCh
			timeCh = nil
			if parent != nil {
				parentCh = parent
				tickCh = nil
			}
			rl.Wait(maxrate)
		case waitCh <- struct{}{}:
			// unblocked a goroutine calling Wait()
			timeCh = nil
			if maxrate != nil {
				if rate := atomic.LoadInt32(maxrate); rate > 0 {
					timer.Reset(time.Second / time.Duration(rate))
					waitCh = nil
					timeCh = timer.C
				}
			}
		case <-timeCh:
			// enough time has passed since last Wait() unblock
			// so that we can safely allow one more to unblock
			waitCh = ticker.waitCh
			timeCh = nil
		case _, ok := <-parentCh:
			// if parentCh is not nil, we require a successful read from it
			if !ok {
				return
			}
			parentCh = nil
			tickCh = ticker.tickCh
		case <-closeCh:
			return
		}
	}
}

// NewTicker returns a Ticker that sends a `struct{}{}`
// at most `*maxrate` times per second on it's C channel.
//
// If counter is not nil, it is incremented every time a
// send is successful.
//
// A nil `maxrate` or a `*maxrate` of zero or less sends
// as quickly as possible.
func NewTicker(maxrate *int32, counter *uint64) *Ticker {
	return NewSubTicker(nil, maxrate, counter)
}

// NewSubTicker returns a channel that reads from another struct{}{}
// channel and then sends a `struct{}{}` at most `*maxrate` times per second,
// but that cannot exceed the parent tick rate.
//
// If counter is not nil, it is incremented every time a
// send is successful.
//
// Use this to make "background" tickers that are less prioritized.
//
// The Ticker is closed when the parent channel closes.
func NewSubTicker(parent <-chan struct{}, maxrate *int32, counter *uint64) *Ticker {
	ticker := &Ticker{
		tickCh:  make(chan struct{}),
		waitCh:  make(chan struct{}),
		closeCh: make(chan struct{}),
	}
	ticker.C = ticker.tickCh
	go ticker.run(ticker.closeCh, parent, maxrate, counter)
	return ticker
}
