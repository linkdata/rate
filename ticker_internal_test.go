package rate

import (
	"math"
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

func TestMaxWorkersCapsOverflowedWorkerRatioProduct(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		maxrate := int32(math.MaxInt32)
		ticker := NewTicker(nil, &maxrate)
		defer ticker.Close()

		ticker.WorkerMax = 10
		ticker.WorkerRatio = math.MaxInt32

		if got := ticker.maxWorkers(); got != ticker.WorkerMax {
			t.Fatalf("maxWorkers() = %d, want %d", got, ticker.WorkerMax)
		}

	})
}
