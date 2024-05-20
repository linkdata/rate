[![build](https://github.com/linkdata/rate/actions/workflows/go.yml/badge.svg)](https://github.com/linkdata/rate/actions/workflows/go.yml)
[![coverage](https://coveralls.io/repos/github/linkdata/rate/badge.svg?branch=main)](https://coveralls.io/github/linkdata/rate?branch=main)
[![goreport](https://goreportcard.com/badge/github.com/linkdata/rate)](https://goreportcard.com/report/github.com/linkdata/rate)
[![Docs](https://godoc.org/github.com/linkdata/rate?status.svg)](https://godoc.org/github.com/linkdata/rate)

## An efficient rate limiter for Go

Because too much of the CPU consumed was `golang.org/x/time/rate.Limiter.Wait()` calling `time.Now()`.

### Differences from `golang.org/x/time/rate` and `time.Ticker`

This package uses ticks-per-second rather than `time.Duration`, and is suitable for high-tickrate applications.
It allows you to change the rate by simply atomically changing an `int32`. It has no practical limitation on the upper rate.
If you need a slower rate than once per second, you're better off using `time.Ticker`.

### Sample usage

One of the more common non-trivial usages of rate limiting is restricting some underlying operation(s)
being utilized by more complex worker goroutines. This package supports those with `Ticker.Worker()`.

Assume we have a channel `taskCh` where we read tasks to process, and then
want spawn worker goroutines that handle them. We want to spawn enough of them
to stay as close to the max rate as possible without starting *too* many of them.

```go
for task := range taskCh {
  // make sure to not alias variables you use in the lambda
  // in case you use Go versions prior to 1.22.
  task := task
  if !ticker.Worker(func() { workerFn(ticker, task) }) {
    // if ticker.Worker() fails to start the worker, it means the Ticker is closed.
    break
  }
}
```
