package db

import (
	"context"
	"time"

	"github.com/xvello/pgbench/internal/stats"
)

// RunQueries executes database queries sequentially and reports latency and errors.
// Latency is measured client-side and is impacted by network latency.
func RunQueries(ctx context.Context, index int, connect ConnectFunc, input <-chan *Query, output chan<- stats.Result) error {
	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	if _, err = conn.Prepare(ctx, TimeBucketQueryName, TimeBucketQueryText); err != nil {
		return err
	}

	for query := range input {
		start := time.Now()
		// Execute the query and discard the result without reading it to better reflect the server-side execution time.
		_, err := conn.Exec(ctx, TimeBucketQueryName, query.Hostname, query.StartTime, query.EndTime)
		output <- stats.Result{
			Worker:  index,
			Latency: time.Since(start),
			Err:     err,
		}
	}

	return conn.Close(ctx)
}
