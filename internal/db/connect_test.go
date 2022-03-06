package db

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/xvello/pgbench/internal/db/mock"
)

func TestWaitFor_OK(t *testing.T) {
	retryWaitDuration = time.Millisecond
	c := gomock.NewController(t)
	conn := mock.NewMockConn(c)
	conn.EXPECT().
		Close(gomock.Any()).
		Return(nil)

	retry := false
	assert.NoError(t, WaitFor(context.Background(), func(ctx context.Context) (Conn, error) {
		if retry {
			return conn, nil
		} else {
			retry = true
			return nil, fmt.Errorf("not ready")
		}
	}))
	assert.True(t, retry)
}

func TestWaitFor_Fail(t *testing.T) {
	retryWaitDuration = time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	retries := uint64(0)
	assert.EqualError(t, WaitFor(ctx, func(ctx context.Context) (Conn, error) {
		atomic.AddUint64(&retries, 1)
		return nil, fmt.Errorf("not ready")
	}), "database unavailable")
	assert.Greater(t, retries, uint64(10))
}
