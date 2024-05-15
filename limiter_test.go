package rate_test

import (
	"testing"
	"time"

	"github.com/linkdata/rate"
)

// maximum duration that times are allowed to exceed expected
const variance = time.Millisecond * 10

func TestLimiter_WaitNilSpins(t *testing.T) {
	var rl rate.Limiter

	now := time.Now()
	for i := 0; i < 10000; i++ {
		rl.Wait(nil)
	}
	if d := time.Since(now); d > variance {
		t.Errorf("%v > %v", d, variance)
	}
}

func TestLimiter_Wait(t *testing.T) {
	tests := []struct {
		name  string
		rate  int32
		count int
	}{
		{
			name:  "zero rate spins",
			rate:  0,
			count: 10000,
		},
		{
			name:  "rate 100",
			rate:  100,
			count: 10,
		},
		{
			name:  "rate exceeds SleepGranularity",
			rate:  rate.SleepGranularity * 100,
			count: rate.SleepGranularity,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rl rate.Limiter
			now := time.Now()
			for i := 0; i < tt.count; i++ {
				rl.Wait(&tt.rate)
			}
			d := time.Since(now)
			var want time.Duration
			if tt.rate > 0 {
				want = time.Second / time.Duration(tt.rate) * time.Duration(tt.count)
			}
			if d < want {
				t.Errorf("%v < %v", d, want)
			}
			if d > want+variance {
				t.Errorf("%v > %v", d, want+variance)
			}
		})
	}
}

func TestLimiter_WaitRateChanges(t *testing.T) {
	var rl rate.Limiter
	now := time.Now()
	rte := int32(rate.SleepGranularity)
	for i := 0; i < 30; i++ {
		if i == 10 {
			rte = 0
		}
		if i == 20 {
			rte = rate.SleepGranularity
		}
		rl.Wait(&rte)
	}
	d := time.Since(now)
	want := (time.Second / rate.SleepGranularity) * 20
	if d < want {
		t.Errorf("%v < %v", d, want)
	}
	if d > want+variance {
		t.Errorf("%v > %v", d, want+variance)
	}
}
