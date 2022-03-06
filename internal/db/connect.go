package db

//go:generate go run -mod=mod github.com/golang/mock/mockgen -package mock -build_flags=-mod=mod -source=connect.go -destination=mock/conn.go

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
)

var retryWaitDuration = time.Second

// Conn is a subset of pgx.Conn's public interface, mocked by MockConn.
type Conn interface {
	Close(ctx context.Context) error
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Prepare(ctx context.Context, name, sql string) (sd *pgconn.StatementDescription, err error)
}

// ConnectFunc is used to instantiate a database connection.
type ConnectFunc func(ctx context.Context) (Conn, error)

// WaitFor tries connecting to the database every second until it succeeds or the context times out.
func WaitFor(ctx context.Context, connect ConnectFunc) error {
	ticker := time.NewTicker(retryWaitDuration)
	defer ticker.Stop()
	for {
		if c, err := connect(ctx); err == nil {
			return c.Close(ctx)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("database unavailable")
		case <-ticker.C:
			continue
		}
	}
}
