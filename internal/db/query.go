package db

import (
	"encoding/csv"
	"fmt"
	"hash/fnv"
	"io"
)

// Exported for use in bench_test.go.
const (
	TimeBucketQueryName = "cpu-buckets"
	TimeBucketQueryText = `SELECT time_bucket('1 minute', ts) as "bucket", min(usage), max(usage)
FROM cpu_usage
WHERE host = $1 AND ts >= $2 AND ts <= $3
GROUP BY bucket
ORDER BY bucket ASC;`
)

// Query holds one set of input parameters.
// Timestamps are assumed valid and kept as strings for simplicity.
type Query struct {
	Hostname  string
	StartTime string
	EndTime   string
}

// Hash returns the consistent hash to be used for worker routing.
func (q *Query) Hash() uint64 {
	hash := fnv.New64()
	_, _ = hash.Write([]byte(q.Hostname))
	return hash.Sum64()
}

// QueryParser parses the input queries one by one.
type QueryParser struct {
	lines *csv.Reader
}

// NewQueryParser returns a new QueryParser.
func NewQueryParser(input io.Reader) (*QueryParser, error) {
	lines := csv.NewReader(input)
	// Skip header line
	if _, err := lines.Read(); err != nil {
		return nil, fmt.Errorf("cannot open input: %w", err)
	}
	return &QueryParser{lines: lines}, nil
}

// Read returns the next query in the input set, or io.EOF when finished.
func (p *QueryParser) Read() (*Query, error) {
	record, err := p.lines.Read()
	if err != nil {
		return nil, err
	}
	if len(record) != 3 {
		return nil, fmt.Errorf("invalid record: %v", record)
	}
	return &Query{
		Hostname:  record[0],
		StartTime: record[1],
		EndTime:   record[2],
	}, nil
}
