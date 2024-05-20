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
	const numTasks = 100
	const wantRate = numTasks / 10
	var result int64
	var workerCount int32
	var highestWorkerCount int32

	maxrate := int32(wantRate)
	ticker := rate.NewTicker(nil, &maxrate)

	// make a task channel and spawn a goroutine sending to it
	taskCh := make(chan int)
	go func() {
		defer close(taskCh)
		for i := 0; i < numTasks; i++ {
			taskCh <- i
		}
	}()

	// define a worker function that just adds and sleeps for a bit
	workerFn := func(i int) {
		for j := 0; j <= i; j++ {
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&result, int64(j))
		}
	}

	// process all the tasks
	for task := range taskCh {
		// make sure to not alias variables you use in the lambda
		// in case you use Go versions prior to 1.22.
		task := task

		if !ticker.Worker(func() {
			// let's keep track of the highest number of concurrent worker goroutines
			defer atomic.AddInt32(&workerCount, -1)
			if count := atomic.AddInt32(&workerCount, 1); count > atomic.LoadInt32(&highestWorkerCount) {
				atomic.StoreInt32(&highestWorkerCount, count)
			}
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
	for i := int64(0); i < numTasks; i++ {
		for j := int64(0); j <= i; j++ {
			wantResult += j
		}
	}

	fmt.Println(result == wantResult, atomic.LoadInt32(&highestWorkerCount) < numTasks/2)
	// Output:
	// true true
}
