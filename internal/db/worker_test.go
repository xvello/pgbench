package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xvello/pgbench/internal/db/mock"
	"github.com/xvello/pgbench/internal/stats"
)

func TestRunQueries(t *testing.T) {
	cases := []struct {
		Hostname     string
		StartTime    string
		EndTime      string
		QueryError   error
		QueryLatency time.Duration
	}{{
		Hostname:     "host_000008",
		StartTime:    "2017-01-01 08:59:22",
		EndTime:      "2017-01-01 09:59:22",
		QueryLatency: time.Duration(5) * time.Millisecond,
	}, {
		Hostname:   "host_000001",
		StartTime:  "A",
		EndTime:    "B",
		QueryError: fmt.Errorf("bad input"),
	}, {
		Hostname:     "host_000003",
		StartTime:    "2017-01-01 04:30:52",
		EndTime:      "2017-01-01 05:30:52",
		QueryLatency: time.Duration(2) * time.Millisecond,
	}}

	c := gomock.NewController(t)
	conn := mock.NewMockConn(c)
	conn.EXPECT().
		Prepare(gomock.Any(), TimeBucketQueryName, TimeBucketQueryText).
		Return(&pgconn.StatementDescription{}, nil)
	conn.EXPECT().
		Close(gomock.Any()).
		Return(nil)

	for _, c := range cases {
		if c.QueryError != nil {
			conn.EXPECT().
				Exec(gomock.Any(), TimeBucketQueryName, c.Hostname, c.StartTime, c.EndTime).
				Return(nil, c.QueryError)
		} else {
			latency := c.QueryLatency
			conn.EXPECT().
				Exec(gomock.Any(), TimeBucketQueryName, c.Hostname, c.StartTime, c.EndTime).
				DoAndReturn(func(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
					time.Sleep(latency)
					return nil, nil
				})
		}
	}

	queryChan := make(chan *Query)
	resultChan := make(chan stats.Result, 3)
	go func() {
		for _, c := range cases {
			queryChan <- &Query{
				Hostname:  c.Hostname,
				StartTime: c.StartTime,
				EndTime:   c.EndTime,
			}
		}
		close(queryChan)
	}()

	assert.NoError(t, RunQueries(context.Background(), 2, func(ctx context.Context) (Conn, error) {
		return conn, nil
	}, queryChan, resultChan))

	for _, c := range cases {
		r, ok := <-resultChan
		require.True(t, ok, "missing expected result")
		if c.QueryError != nil {
			assert.EqualError(t, r.Err, c.QueryError.Error())
		} else {
			assert.Nil(t, r.Err)
			assert.InDelta(t, c.QueryLatency.Seconds(), r.Latency.Seconds(), time.Millisecond.Seconds())
		}
	}
}
