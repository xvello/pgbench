package bench

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/golang/mock/gomock"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xvello/pgbench/internal/db"
	"github.com/xvello/pgbench/internal/db/mock"
)

const (
	inputFile   = "testdata/query_params.csv"
	queryCount  = 200
	workerCount = 4
)

// TestRunBenchmark is a functional test of the whole pipeline running 200 queries, with only the DB mocked.
func TestRunBenchmark(t *testing.T) {
	c := gomock.NewController(t)
	conn := mock.NewMockConn(c)
	execCount := uint64(0)

	conn.EXPECT().
		Prepare(gomock.Any(), db.TimeBucketQueryName, db.TimeBucketQueryText).
		Return(&pgconn.StatementDescription{}, nil).
		Times(workerCount)
	conn.EXPECT().
		Exec(gomock.Any(), db.TimeBucketQueryName, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
			n := atomic.AddUint64(&execCount, 1)
			if n%100 == 1 {
				return nil, fmt.Errorf("one percent error rate")
			}
			if n%10 == 0 {
				time.Sleep(time.Millisecond)
			}
			time.Sleep(time.Microsecond)
			return pgconn.CommandTag{}, nil
		}).Times(queryCount)
	conn.EXPECT().
		Close(gomock.Any()).
		Return(nil).
		Times(workerCount)

	cmd := &BenchmarkCommand{
		Input:       inputFile,
		Concurrency: workerCount,
	}
	stats, err := cmd.runBench(&kong.Context{}, func(ctx context.Context) (db.Conn, error) {
		return conn, nil
	})
	require.NoError(t, err)

	// Check concurrency and work sharing
	assert.EqualValues(t, workerCount, stats.BenchConcurrency)
	assert.Greater(t, stats.BenchDuration, 1.)
	require.Len(t, stats.QueriesPerWorker, 4)
	for i, v := range stats.QueriesPerWorker {
		// Check that each worker processed at least half of an even spread
		assert.Greater(t, v, uint64(queryCount/workerCount/2), "not enough queries processed by worker", i)
	}

	// 1% query error rate
	assert.EqualValues(t, queryCount*.99, stats.QueriesOk)
	assert.EqualValues(t, queryCount*.01, stats.QueriesErr)

	// All queries sleep at least one microsecond
	assert.Greater(t, stats.Min, 0.001)
	assert.Greater(t, stats.Mean, 0.001)
	assert.Greater(t, stats.Median, 0.001)

	// 10% of queries sleep a millisecond -> p50 unaffected, p90 affected
	assert.Less(t, stats.Median, 1.)
	assert.Greater(t, stats.P90, 1.)
	assert.Greater(t, stats.P95, 1.)
	assert.Greater(t, stats.P99, 1.)
	assert.Greater(t, stats.Max, 1.)
	assert.Greater(t, stats.Sum, 20.)
}
