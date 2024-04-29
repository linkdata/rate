[![build](https://github.com/linkdata/rate/actions/workflows/go.yml/badge.svg)](https://github.com/linkdata/rate/actions/workflows/go.yml)
[![coverage](https://coveralls.io/repos/github/linkdata/rate/badge.svg?branch=main)](https://coveralls.io/github/linkdata/rate?branch=main)
[![goreport](https://goreportcard.com/badge/github.com/linkdata/rate)](https://goreportcard.com/report/github.com/linkdata/rate)
[![Docs](https://godoc.org/github.com/linkdata/rate?status.svg)](https://godoc.org/github.com/linkdata/rate)

## An efficient rate limiter for Go

Because too much of the CPU consumed was `golang.org/x/time/rate.Limiter.Wait()` calling `time.Now()`.
