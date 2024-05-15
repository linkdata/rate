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
	noneElapsed := time.Since(now)

	// instead of calling now = time.Now(), which can be slow
	now = now.Add(noneElapsed)

	// This waits at least 1ms.
	limiter.Wait(&maxrate)
	someElapsed := time.Since(now)

	fmt.Println(noneElapsed < someElapsed, someElapsed >= time.Millisecond)
	// Output:
	// true true
}
