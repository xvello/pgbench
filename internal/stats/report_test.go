package stats

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadResults(t *testing.T) {
	resultChan := make(chan Result)
	go func() {
		resultChan <- Result{Err: fmt.Errorf("one error")}
		for i := 1; i < 13; i++ {
			resultChan <- Result{
				Worker:  i % 4,
				Latency: time.Duration(i) * time.Millisecond,
			}
		}
		resultChan <- Result{Err: fmt.Errorf("another error")}
		close(resultChan)
	}()

	report := ReadResults(4, resultChan)
	assert.Greater(t, report.BenchDuration, 0.)
	report.BenchDuration = 0

	assert.EqualValues(t, &Report{
		BenchConcurrency: 4,
		QueriesPerWorker: []uint64{5, 3, 3, 3},
		QueriesErr:       2,
		QueriesOk:        12,
		Min:              1,
		Mean:             6.5,
		Median:           6,
		P90:              11,
		P95:              12,
		P99:              12,
		Max:              12,
		Sum:              78,
	}, report)
}

func TestReport_Print(t *testing.T) {
	report := &Report{
		BenchConcurrency: 4,
		BenchDuration:    12.34567890,
		QueriesPerWorker: []uint64{5, 3, 3, 3},
		QueriesErr:       2,
		QueriesOk:        12,
		Min:              1.12345678,
		Mean:             6.5,
		Median:           6,
		P90:              11,
		P95:              12,
		P99:              12,
		Max:              12,
		Sum:              78,
	}

	buffer := strings.Builder{}
	assert.NoError(t, report.Print(&buffer, false))
	assert.Equal(t, `
Benchmark duration: 12.346 ms
Concurrency Level:  4 workers
Queries per worker: [5 3 3 3]

Completed queries:  12
Failed queries:     2 (1% error rate)

Measured query latency:
  Min:    1.123 ms
  Mean:   6.500 ms
  Median: 6.000 ms
  p90:    11.000 ms
  p95:    12.000 ms
  p99:    12.000 ms
  Max:    12.000 ms
  Sum:    78.000 ms
`, buffer.String())

	buffer.Reset()
	assert.NoError(t, report.Print(&buffer, true))
	assert.Equal(t, `{
  "bench_concurrency": 4,
  "bench_duration": 12.3456789,
  "queries_per_worker": [
    5,
    3,
    3,
    3
  ],
  "queries_error": 2,
  "queries_ok": 12,
  "min_latency": 1.12345678,
  "mean_latency": 6.5,
  "median_latency": 6,
  "p90_latency": 11,
  "p95_latency": 12,
  "p99_latency": 12,
  "max_latency": 12,
  "latency_sum": 78
}
`, buffer.String())
}
