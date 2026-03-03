package rate

import (
	"sync/atomic"
	"time"
)

// SleepGranularity 500 implies that the time.Sleep() granularity is at least 2ms
const SleepGranularity = 500

// Limiter provides efficient rate limiting. The zero value is immediately usable.
//
// A Limiter is not safe to use from multiple goroutines simultaneously.
type Limiter struct {
	lastEnded time.Time
	sleepDur  time.Duration
	maxRate   int32
	count     int32
	countMax  int32
	CloseCh   <-chan struct{} // may be nil, if set can break Wait() before waiting is complete
}

// Wait sleeps at least long enough to ensure that Wait cannot be
// called more than `*maxrate` times per second.
//
// A nil `maxrate` or a `*maxrate` of zero or less doesn't wait at all.
func (rl *Limiter) Wait(maxrate *int32) {
	if maxrate != nil {
		if newRate := atomic.LoadInt32(maxrate); newRate != rl.maxRate {
			rl.maxRate = newRate
			rl.lastEnded = time.Now()
			rl.count = 0
			if newRate > 0 {
				countMax := max(newRate/SleepGranularity, 1)
				rl.countMax = countMax
				rl.sleepDur = time.Second * time.Duration(rl.countMax) / time.Duration(newRate)
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
				if toSleep := rl.sleepDur - elapsed; toSleep > 0 {
					if toSleep > (time.Second/SleepGranularity)*10 && rl.CloseCh != nil {
						select {
						case t := <-time.After(toSleep):
							rl.lastEnded = t
						case <-rl.CloseCh:
							rl.lastEnded = time.Now()
						}
					} else {
						time.Sleep(toSleep)
						rl.lastEnded = rl.lastEnded.Add(toSleep)
					}
				}
			}
		}
	}
}
