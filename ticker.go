package rate

import "context"

// NewTicker returns a channel that sends a `struct{}{}`
// at most `*maxrate` times per second.
//
// A nil `maxrate` or a `*maxrate` of zero or less sends
// as quickly as possible.
//
// The channel is closed when the context is done.
func NewTicker(ctx context.Context, maxrate *int32) <-chan struct{} {
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
			rl.Wait(maxrate)
		}
	}()
	return ch
}

// NewSubTicker returns a channel that reads from another struct{}{}
// channel and then sends a `struct{}{}` at most `*maxrate` times per second,
// but that cannot exceed the parent tick rate.
//
// Use this to make "background" tickers that are less prioritized.
//
// The channel is closed when the parent channel is closed.
func NewSubTicker(parent <-chan struct{}, maxrate *int32) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		var rl Limiter
		for range parent {
			ch <- struct{}{}
			rl.Wait(maxrate)
		}
	}()
	return ch
}
