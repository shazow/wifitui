package main

import "time"

// shortDurationValue and longDurationValue can be overridden at compile time,
// for example:
// go test -ldflags "-X main.shortDurationValue=5ms -X main.longDurationValue=25ms"
var (
	shortDurationValue = "10ms"
	longDurationValue  = "50ms"

	shortDuration = mustParseTestDuration(shortDurationValue)
	longDuration  = mustParseTestDuration(longDurationValue)
)

func mustParseTestDuration(value string) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		panic("invalid test duration: " + value + ": " + err.Error())
	}
	return d
}
