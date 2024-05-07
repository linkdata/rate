package rate

import (
	"runtime"
	"sync/atomic"
	"time"
)

type Ticker struct {
	C       <-chan struct{} // sends a struct{}{} at most maxrate times per second
	tickCh  chan struct{}   // source for C
	waitCh  chan struct{}   // source for Wait
	counter *uint64
	closing int32
	stopped int32
}

// Close stops the Ticker and frees resources.
//
// It is safe to call multiple times or concurrently.
func (ticker *Ticker) Close() {
	if atomic.CompareAndSwapInt32(&ticker.closing, 0, 1) {
		defer close(ticker.tickCh)
		defer close(ticker.waitCh)
		for atomic.LoadInt32(&ticker.stopped) != 1 {
			select {
			case <-ticker.waitCh:
			default:
				runtime.Gosched()
			}
		}
	}
}

// Wait delays until the next tick is available.
func (ticker *Ticker) Wait() {
	<-ticker.waitCh
}

func (ticker *Ticker) run(parent <-chan struct{}, maxrate *int32, counter *uint64) {
	defer func() {
		atomic.StoreInt32(&ticker.stopped, 1)
		ticker.Close()
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

	for atomic.LoadInt32(&ticker.closing) == 0 {
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
		counter: counter,
	}
	ticker.C = ticker.tickCh
	go ticker.run(parent, maxrate, counter)
	return ticker
}
