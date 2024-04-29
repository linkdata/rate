package rate

import "context"

// NewTicker returns a channel that sends a `struct{}{}`
// at most `*rate` times per second.
//
// A nil `rate` or a `*rate` of zero or less sends
// as quickly as possible.
//
// The channel is closed when the context is done.
func NewTicker(ctx context.Context, rate *int32) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		var rl Limiter
		for {
			select {
			case <-ctx.Done():
				return
			case ch <- struct{}{}:
			}
			rl.Wait(rate)
		}
	}()
	return ch
}

// NewSubTicker returns a channel that reads from another struct{}{}
// channel and then sends a `struct{}{}` at most `*rate` times per second,
// but that cannot exceed the parent tick rate.
//
// Use this to make "background" tickers that are less prioritized.
//
// The channel is closed when the parent channel is closed.
func NewSubTicker(parent <-chan struct{}, rate *int32) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		var rl Limiter
		for range parent {
			ch <- struct{}{}
			rl.Wait(rate)
		}
	}()
	return ch
}
