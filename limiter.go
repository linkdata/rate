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
	maxRate   int32
	count     int32
	countMax  int32
}

// Wait sleeps at least long enough to ensure that Wait cannot be
// called more than `*maxrate` times per second.
//
// Returns the duration slept.
//
// A nil `maxrate` or a `*maxrate` of zero or less doesn't wait at all.
func (rl *Limiter) Wait(maxrate *int32) (slept time.Duration) {
	if maxrate != nil {
		if newRate := atomic.LoadInt32(maxrate); newRate != rl.maxRate {
			rl.maxRate = newRate
			rl.lastEnded = time.Now()
			rl.count = 0
			if newRate > 0 {
				countMax := newRate / sleepGranularity
				if countMax < 1 {
					countMax = 1
				}
				rl.countMax = countMax
				rl.sleepDur = time.Second / time.Duration(newRate/rl.countMax)
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
				if slept = rl.sleepDur - elapsed; slept > 0 {
					rl.lastEnded = rl.lastEnded.Add(slept)
					time.Sleep(slept)
				}
			}
		}
	}
	return
}
