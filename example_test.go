package rate_test

import (
	"fmt"
	"time"

	"github.com/linkdata/rate"
)

func ExampleLimiter_Wait() {
	var limiter rate.Limiter
	var now time.Time
	maxrate := int32(1000)

	// This doesn't wait at all since we haven't waited for anything yet.
	now = time.Now()
	limiter.Wait(&maxrate)
	zeroElapsed := time.Since(now)

	// This waits at least 1ms.
	maxrate = 1000
	now = time.Now()
	limiter.Wait(&maxrate)
	someElapsed := time.Since(now)

	fmt.Println(zeroElapsed < someElapsed, someElapsed >= time.Millisecond)
	// Output:
	// true true
}
