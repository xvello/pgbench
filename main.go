package main

import (
	"github.com/alecthomas/kong"
	"github.com/xvello/pgbench/internal/bench"
)

func main() {
	cmd := &bench.BenchmarkCommand{}
	k := kong.Parse(cmd)
	k.FatalIfErrorf(cmd.Run(k))
}
