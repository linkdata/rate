package rate

import (
	"testing"
	"time"
)

func TestNewTickerDefaultsTimerIntervalWhenNonPositive(t *testing.T) {
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
}
