package rate_test

import (
	"fmt"
	"time"

	"github.com/linkdata/rate"
)

func ExampleLimiter_Wait() {
	var limiter rate.Limiter
	maxrate := int32(1000)
	now := time.Now()

	// This doesn't wait at all since we haven't waited for anything yet.
	limiter.Wait(&maxrate)
	noneElapsed := time.Since(now)

	// Instead of calling now = time.Now(), which can be slow, just add noneElapsed.
	now = now.Add(noneElapsed)

	// This waits at least 1ms because the maxrate is 1000.
	limiter.Wait(&maxrate)
	someElapsed := time.Since(now)

	fmt.Println(noneElapsed < someElapsed, someElapsed >= time.Second/time.Duration(maxrate))
	// Output:
	// true true
}
