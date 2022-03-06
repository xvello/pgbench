package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"text/template"
	"time"

	"github.com/beorn7/perks/quantile"
)

const outputTemplateText = `
Benchmark duration: {{ formatMs .BenchDuration }}
Concurrency Level:  {{ .BenchConcurrency }} workers
Queries per worker: {{ printf "%v" .QueriesPerWorker }}

Completed queries:  {{ .QueriesOk }}
Failed queries:     {{ .QueriesErr }} ({{ errorRate . }}% error rate)

Query performance:
  Min:    {{ formatMs .Min }}
  Mean:   {{ formatMs .Mean }}
  Median: {{ formatMs .Median }}
  p90:    {{ formatMs .P90 }}
  p95:    {{ formatMs .P95 }}
  p99:    {{ formatMs .P99 }}
  Max:    {{ formatMs .Max }}
  Sum:    {{ formatMs .Sum }}
`

// Result holds the execution result for one query, to be aggregated into a Report.
type Result struct {
	Worker  int
	Latency time.Duration
	Err     error
}

// Report holds raw data for the benchmark report. Durations are in milliseconds.
type Report struct {
	BenchConcurrency uint32   `json:"bench_concurrency"`
	BenchDuration    float64  `json:"bench_duration"`
	QueriesPerWorker []uint64 `json:"queries_per_worker"`
	QueriesErr       uint64   `json:"queries_error"`
	QueriesOk        uint64   `json:"queries_ok"`
	Min              float64  `json:"min_latency"`
	Mean             float64  `json:"mean_latency"`
	Median           float64  `json:"median_latency"`
	P90              float64  `json:"p90_latency"`
	P95              float64  `json:"p95_latency"`
	P99              float64  `json:"p99_latency"`
	Max              float64  `json:"max_latency"`
	Sum              float64  `json:"latency_sum"`
}

// ReadResults consumes a channel of Result and returns the aggregated benchmark Report.
func ReadResults(concurrency uint32, c <-chan Result) *Report {
	start := time.Now()
	stats := Report{
		BenchConcurrency: concurrency,
		QueriesPerWorker: make([]uint64, concurrency),
		Min:              math.MaxFloat64,
		// Keep other fields at zero
	}
	quantiles := quantile.NewTargeted(map[float64]float64{
		0.50: 0.005,
		0.90: 0.001,
		0.95: 0.0005,
		0.99: 0.0001,
	})

	for r := range c {
		if r.Worker >= 0 && r.Worker < len(stats.QueriesPerWorker) {
			stats.QueriesPerWorker[r.Worker]++
		}
		if r.Err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "execution error: %s\n", r.Err)
			stats.QueriesErr++
			continue
		}
		stats.QueriesOk++

		latencyMs := durationToMs(r.Latency)
		quantiles.Insert(latencyMs)
		stats.Sum += latencyMs
		if latencyMs > stats.Max {
			stats.Max = latencyMs
		}
		if latencyMs < stats.Min {
			stats.Min = latencyMs
		}
	}

	stats.BenchDuration = durationToMs(time.Since(start))
	stats.Mean = stats.Sum / float64(stats.QueriesOk)
	stats.Median = quantiles.Query(0.50)
	stats.P90 = quantiles.Query(0.90)
	stats.P95 = quantiles.Query(0.95)
	stats.P99 = quantiles.Query(0.99)

	return &stats
}

// Print can be used to output the report, either in text or json format.
func (s *Report) Print(w io.Writer, toJson bool) error {
	if toJson {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(s)
	}
	tpl, err := template.New("stats").Funcs(template.FuncMap{
		"formatMs": func(v float64) string {
			return fmt.Sprintf("%.3f ms", v)
		},
		"errorRate": func(s *Report) int {
			// Return error rate rounded up to percent
			return int(math.Ceil(float64(s.QueriesErr) / float64(s.QueriesOk+s.QueriesErr)))
		},
	}).Parse(outputTemplateText)
	if err != nil {
		return err
	}

	return tpl.Execute(w, s)
}

func durationToMs(d time.Duration) float64 {
	return float64(d) / 1e6
}
