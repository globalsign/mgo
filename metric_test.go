package mgo

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestCounterVec_Success(t *testing.T) {
	counter := CounterVec()
	assert.NotNil(t, counter)
	assert.IsType(t, &prometheus.CounterVec{}, counter)
}
