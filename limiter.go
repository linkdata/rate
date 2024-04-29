package rate

import (
	"sync/atomic"
	"time"
)

// sleepGranularity 500 implies that the time.Sleep() granularity is at least 2ms
const sleepGranularity = 500

// Limiter provides efficient rate limiting. The zero value is immediately usable.
//
// A Limiter is not safe to use from multiple goroutines simultaneously.
type Limiter struct {
	lastEnded time.Time
	sleepDur  time.Duration
	currRate  int32
	count     int32
	countMax  int32
}

// Wait sleeps at least long enough to ensure that the
// given `*rate` of events per second is not exceeded.
//
// A nil `rate` or a `*rate` of zero or less doesn't wait at all.
func (rl *Limiter) Wait(rate *int32) {
	if rate != nil {
		if wantRate := atomic.LoadInt32(rate); wantRate != rl.currRate {
			rl.currRate = wantRate
			rl.lastEnded = time.Now()
			rl.count = 0
			if wantRate > 0 {
				countMax := wantRate / sleepGranularity
				if countMax < 1 {
					countMax = 1
				}
				rl.countMax = countMax
				rl.sleepDur = time.Second / time.Duration(wantRate/rl.countMax)
			} else {
				rl.countMax = 0
				rl.sleepDur = 0
			}
		}
		if rl.countMax > 0 {
			if rl.count++; rl.count >= rl.countMax {
				rl.count = 0
				elapsed := time.Since(rl.lastEnded)
				rl.lastEnded = rl.lastEnded.Add(elapsed)
				if nextSleep := rl.sleepDur - elapsed; nextSleep > 0 {
					rl.lastEnded = rl.lastEnded.Add(nextSleep)
					time.Sleep(nextSleep)
				}
			}
		}
	}
}
