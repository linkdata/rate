package rate

import (
	"runtime"
	"sync/atomic"
	"time"
)

type Ticker struct {
	C       <-chan struct{}
	ch      chan struct{}
	waiting int32
	closing int32
	stopped int32
}

// Close stops the Ticker and frees resources.
//
// It is safe to call multiple times or concurrently.
func (ticker *Ticker) Close() {
	if atomic.CompareAndSwapInt32(&ticker.closing, 0, 1) {
		defer close(ticker.ch)
		for atomic.LoadInt32(&ticker.stopped) != 1 {
			select {
			case <-ticker.ch:
			default:
			}
			runtime.Gosched()
		}
	}
}

// Waiting returns true if the Ticker is currently waiting for the next tick to be available.
//
// Note that this is not safe from data races -- even if this returns false, the next read
// from the channel may block.
func (ticker *Ticker) Waiting() bool {
	return atomic.LoadInt32(&ticker.waiting) != 0
}

// AddTick adds a single tick to the Ticker, retrying with the
// given interval until it succeeds or the Ticker is closed.
func (ticker *Ticker) AddTick(d time.Duration) {
	for atomic.LoadInt32(&ticker.stopped)+atomic.LoadInt32(&ticker.closing) == 0 {
		select {
		case ticker.ch <- struct{}{}:
			return
		default:
		}
		time.Sleep(d)
	}
}

func (ticker *Ticker) run(parent <-chan struct{}, maxrate *int32, counter *uint64) {
	defer func() {
		atomic.StoreInt32(&ticker.stopped, 1)
		ticker.Close()
	}()
	var rl Limiter
	for atomic.LoadInt32(&ticker.closing) == 0 {
		if parent != nil {
			if _, ok := <-parent; !ok {
				break
			}
		}
		atomic.StoreInt32(&ticker.waiting, 0)
		ticker.ch <- struct{}{}
		atomic.StoreInt32(&ticker.waiting, 1)
		if counter != nil {
			atomic.AddUint64(counter, 1)
		}
		rl.Wait(maxrate)
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
func NewTicker(maxrate *int32, counter *uint64) (ticker *Ticker) {
	ch := make(chan struct{})
	ticker = &Ticker{C: ch, ch: ch}
	go ticker.run(nil, maxrate, counter)
	return
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
func NewSubTicker(parent <-chan struct{}, maxrate *int32, counter *uint64) (ticker *Ticker) {
	ch := make(chan struct{})
	ticker = &Ticker{C: ch, ch: ch}
	go ticker.run(parent, maxrate, counter)
	return
}
