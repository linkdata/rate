package rate

import (
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestNewTickerDefaultsTimerIntervalWhenNonPositive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		original := TickerTimerInterval
		TickerTimerInterval = 0
		defer func() {
			TickerTimerInterval = original
		}()

		ticker := NewTicker(nil, nil)
		defer ticker.Close()

		if ticker.timerInterval != time.Second {
			t.Fatalf("timerInterval is %v, wanted %v", ticker.timerInterval, time.Second)
		}

	})
}

func TestCloseWaitsForWaitingCounter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ticker := NewTicker(nil, nil)
		atomic.StoreInt32(&ticker.waiting, 1)

		go func() {
			runtime.Gosched()
			atomic.StoreInt32(&ticker.waiting, 0)
		}()

		ticker.Close()
	})
}
