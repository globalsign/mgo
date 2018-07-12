package mgo

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	once    sync.Once
	counter *prometheus.CounterVec
)

func init() {
	once.Do(func() {
		counter = newCounterVec()
	})
}

// CounterVec returns prometheus.CounterVec.
// It is used by mysql driver to count how many requests fall for earch circuit breaker state.
// Developers can export this counter to their project so they can monitor circuit breaker usage.
func CounterVec() *prometheus.CounterVec {
	return counter
}

func newCounterVec() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "entity_total",
		Help: "number of entity recorded based on its status",
	}, []string{"entity", "status"})
}
