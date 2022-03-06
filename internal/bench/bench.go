package bench

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/jackc/pgx/v4"
	"github.com/xvello/pgbench/internal/db"
	"github.com/xvello/pgbench/internal/stats"
)

const (
	workerChannelSize = 32
	resultChannelSize = 32
)

type BenchmarkCommand struct {
	Input        string        `default:"-" help:"input file to use, defaults to '-' for stdin" arg:"" type:"existingfile"`
	Concurrency  uint32        `default:"4" help:"number of connections to spread the queries across"`
	DatabaseUrl  string        `env:"DATABASE_URL" help:"postgres connection string"`
	DatabaseWait time.Duration `default:"30s" help:"wait until the database accepts connections"`
	Json         bool          `help:"output the report in JSON format"`
	Profile      bool          `help:"record pprof profiles"`
}

func (c *BenchmarkCommand) Run(k *kong.Context) error {
	if c.Concurrency < 1 {
		return fmt.Errorf("worker count must be at least 1")
	}

	if c.Profile {
		f, err := os.Create("cpu.pprof")
		k.FatalIfErrorf(err)
		defer func() {
			pprof.StopCPUProfile()
			k.FatalIfErrorf(f.Close())
			f, err = os.Create("mem.pprof")
			k.FatalIfErrorf(err)
			runtime.GC()
			k.FatalIfErrorf(pprof.WriteHeapProfile(f))
			k.FatalIfErrorf(f.Close())
		}()
		k.FatalIfErrorf(pprof.StartCPUProfile(f))
	}

	connect := func(ctx context.Context) (db.Conn, error) {
		return pgx.Connect(ctx, c.DatabaseUrl)
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.DatabaseWait)
	defer cancel()
	k.FatalIfErrorf(db.WaitFor(ctx, connect))

	report, err := c.runBench(k, connect)
	if err != nil {
		return err
	}
	return report.Print(os.Stdout, c.Json)
}

func (c *BenchmarkCommand) buildInput() (io.Reader, error) {
	if c.Input == "-" {
		return os.Stdin, nil
	}
	file, err := os.Open(c.Input)
	if err != nil {
		return nil, fmt.Errorf("cannot open input file: %w", err)
	}
	return file, nil
}

func (c *BenchmarkCommand) runBench(k *kong.Context, cf db.ConnectFunc) (*stats.Report, error) {
	ctx := context.Background()

	input, err := c.buildInput()
	if err != nil {
		return nil, err
	}

	queries, err := db.NewQueryParser(input)
	if err != nil {
		return nil, err
	}

	resultChan := make(chan stats.Result, resultChannelSize)
	workerChan := make([]chan *db.Query, c.Concurrency)
	workerGroup := sync.WaitGroup{}
	workerGroup.Add(int(c.Concurrency))

	// Spawn database workerCount
	for i := range workerChan {
		i := i
		c := make(chan *db.Query, workerChannelSize)
		workerChan[i] = c
		go func() {
			k.FatalIfErrorf(db.RunQueries(ctx, i, cf, c, resultChan))
			workerGroup.Done()
		}()
	}

	// Spawn a goroutine to close the results channel when all workerCount have returned
	go func() {
		workerGroup.Wait()
		close(resultChan)
	}()

	// Spawn a goroutine to feed queries to the workerCount
	go func() {
		for {
			q, e := queries.Read()
			if e == io.EOF {
				break
			}
			if e != nil { // Skip and report parsing errors
				resultChan <- stats.Result{Err: e}
				continue
			}
			workerChan[int(q.Hash()%uint64(c.Concurrency))] <- q
		}
		for _, c := range workerChan {
			close(c)
		}
	}()

	// Collect results and build the statistics report
	return stats.ReadResults(c.Concurrency, resultChan), nil
}
