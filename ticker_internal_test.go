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

func TestRunReturnsWhenParentAlreadyClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parent := NewTicker(nil, nil)
		parent.Close()

		ticker := NewTicker(parent, nil)
		defer ticker.Close()

		if _, ok := <-ticker.C; ok {
			t.Fatal("ticker channel remained open for already closed parent")
		}

		if !ticker.IsClosed() {
			t.Fatal("ticker did not report closed")
		}
	})
}

func TestRunReturnsWhenParentChannelClosesBeforeCloseSignal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parentC := make(chan struct{})
		close(parentC)

		parent := &Ticker{
			C:       parentC,
			closeCh: make(chan struct{}),
		}
		ticker := &Ticker{
			timerInterval: time.Second,
			tickCh:        make(chan struct{}),
			closeCh:       make(chan struct{}),
		}
		ticker.C = ticker.tickCh

		ticker.run(ticker.closeCh, parent)

		if _, ok := <-ticker.C; ok {
			t.Fatal("ticker channel remained open after closed parent channel")
		}

		if !ticker.IsClosed() {
			t.Fatal("ticker did not report closed")
		}
	})
}
