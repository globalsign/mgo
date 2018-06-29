package mgo

import (
	"testing"
	"time"
)

func TestCoarseTimeProvider(t *testing.T) {
	t.Skip("highly reliant on the scheduler to pass")

	const granularity = 50 * time.Millisecond

	ct := newcoarseTimeProvider(granularity)
	defer ct.Close()

	start := ct.Now().Unix()
	time.Sleep(time.Second)

	got := ct.Now().Unix()
	if got <= start {
		t.Fatalf("got %d, expected at least %d", got, start)
	}
}
