package rate_test

import (
	"fmt"
	"sync/atomic"
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

func ExampleTicker_Worker() {
	const numTasks = 20
	const wantRate = numTasks * 10
	var result int64

	maxrate := int32(wantRate)
	ticker := rate.NewTicker(nil, &maxrate)
	defer ticker.Close()

	// make a task channel and spawn a goroutine sending to it
	taskCh := make(chan int)
	go func() {
		defer close(taskCh)
		for i := range numTasks {
			taskCh <- i
		}
	}()

	// define a worker function that just adds and sleeps for a bit
	workerFn := func(i int) {
		<-ticker.C
		for j := 0; j <= i; j++ {
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&result, int64(j))
		}
	}

	// process all the tasks
	for task := range taskCh {
		if !ticker.Worker(func() {
			// call the worker.
			workerFn(task)
		}) {
			// if ticker.Worker() fails to start the worker, it means the Ticker is closed.
			break
		}
	}

	// wait for the workers to be done
	for ticker.WorkerCount() != 0 {
		time.Sleep(time.Millisecond)
	}

	// calculate the expected result
	var wantResult int64
	for i := range int64(numTasks) {
		for j := int64(0); j <= i; j++ {
			wantResult += j
		}
	}

	fmt.Println(result == wantResult, ticker.WorkerCount() == 0)
	// Output:
	// true true
}
