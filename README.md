# Timeseries query benchmarking tool

This command-line tool is designed to run a set of `SELECT` queries on a TimescaleDB hypertable, with configurable
concurrency. It reports the query error rate and latency.

```
Usage: pgbench [<input>]

Arguments:
  [<input>]    input file to use, defaults to '-' for stdin

Flags:
  -h, --help                   Show context-sensitive help.
      --concurrency=4          number of connections to spread the queries across
      --database-url=STRING    postgres connection string ($DATABASE_URL)
      --database-wait=30s      wait until the database accepts connections
      --json                   output the report in JSON format
      --profile                record pprof profiles
```

**Please note:** this tool measures the query latency as seen on the client-side, which includes the network latency
to the server. The difference can be ignored when running the database on the same host, but needs to be considered
if benchmarking a cloud instance. See the improvement notes below for more context.

## Usage instructions

### Using `docker-compose`

Run `make docker-run` to start a containerized Postgres instance, load the test corpus and run the benchmark with
four concurrent connections. It will output a text report after all the queries are executed:

```
pgbench_1    | Benchmark duration: 150.538 ms
pgbench_1    | Concurrency Level:  4 workers
pgbench_1    | Queries per worker: [44 44 52 60]
pgbench_1    | 
pgbench_1    | Completed queries:  200
pgbench_1    | Failed queries:     0 (0% error rate)
pgbench_1    | 
pgbench_1    | Measured query latency:
pgbench_1    |   Min:    1.266 ms
pgbench_1    |   Mean:   2.325 ms
pgbench_1    |   Median: 2.058 ms
pgbench_1    |   p90:    3.106 ms
pgbench_1    |   p95:    3.684 ms
pgbench_1    |   p99:    7.923 ms
pgbench_1    |   Max:    9.848 ms
pgbench_1    |   Sum:    465.013 ms
xvello-pgbench_pgbench_1 exited with code 0
```

Alternatively, you can run the command with custom arguments. For example, to use a single connection and output
the report in JSON format:

```bash
docker-compose run pgbench /pgbench query_params.csv --concurrency=1 --json
```
```json
{
  "bench_concurrency": 1,
  "bench_duration": 323.029368,
  "queries_per_worker": [
    200
  ],
  "queries_error": 0,
  "queries_ok": 200,
  "min_latency": 1.334023,
  "mean_latency": 1.543599255000001,
  "median_latency": 1.415917,
  "p90_latency": 1.778344,
  "p95_latency": 2.008814,
  "p99_latency": 3.427357,
  "max_latency": 5.907161,
  "latency_sum": 308.71985100000023
}
```

The queries are sourced from the `data/query_params.csv`, which can be modified between runs. If you change
the database schema or contents (`data/01-create-schema.sql` and `data/cpu_usage.csv`), you need to run
`make docker-clean` to trigger the database provisioning logic on the next run.

### Using `go run`

Assuming the `homework.cpu_usage` table is correctly set on a local PSQL instance, you can run
`go run . data/query_params.csv` to run the benchmark. If the database is not accessible via peer authentication on
a unix socket, set the `--database-url` parameter or `DATABASE_URL` environment variable with a valid PostgreSQL
connect string.

This allows you to pipe data in and out of the program, like so:
```bash
cat data/query_params.csv | go run . --json | jq '.p95_latency'
2.661737
```

### Interpreting the results

- All queries are executed, even if some fail. Unless your data set includes purposely erroneous queries, a non-zero
error rate should be investigated before using the results.

- The latency measurement(s) to look for depend on your use case:
    - For batch operations, the `average` value will be a good indication of the processing throughput of your service,
    - For interactive services, watching the `p90` / `p95` percentiles will give a better measurement of the user
      experience under load. It might also surface pathological cases to be investigated.

- Due to the hostname locality constraint, the desired concurrency might not be achieved.
If the "Queries per worker" report line shows significant disparities between the workers, the query set should
be reworked to achieve a higher concurrency. For example, the provided `query_params.csv` file only targets 10
hostnames, so running with a concurrency of 16 will result in idle connections with zero queries:

```
Queries per worker: [27 19 18 23 17 25 17 22 0 0 0 0 0 0 17 15]
```

## Improvement notes

### Execution time vs Query latency

Unlike some other data stores, PostgreSQL does not report the query execution time in its response. Because
using `EXPLAIN` would incur additional work and skew the results, `pgbench` measures the time the query
took, as seen by the client, this is why the report uses the term `latency`. When benchmarking a non-local
database, the network latency needs to be accounted for. Running `pgbench` on the same cloud availability-zone
would help keeping it stable.

If a finer measurement was required, I would investigate whether it could be provided via a PSQL extension:
the extension would hook into the execution flow, keep track or per-table execution statistics, and expose it
as a function or a view, for `pgbench` to retrieve it.

The results could also be rendered invalid by `pgbench` being the bottleneck. The `--pprof` parameter enables
profiling of the program, to be used when making updates to the code.

### Impact of the caches / background jobs

- `make docker-run` restarts the database on each run to reduce the impact of caching and make the results
more reproducible, but more investigation would be needed for me to be more confident that the results are
not skewed by caching.

- We could implement a `--repeat` parameter to the CLI to loop on the query corpus several
times and benchmark the database on a sustained load. The first run(s) could even be excluded from the report
as a "warmup" phase. My intuition is that this is less important with PSQL than JVM-based data stores, where
the garbage collection can have a significant impact on the tail latency.

### User experience improvements

- For simplicity, `pgbench` does not output partial results while the benchmark is running. For bigger query sets,
or when implementing the `--repeat` logic, I would show at least basic progress information to the user.

- Another shortcut I took is direct use of `fmt.Fprintf` to output errors. A proper logging library, with
configurable logging levels, would improve the UX.

- If the database is present but empty, queries succeed with abnormally high performance. To protect against this
we could `SELECT COUNT(*) FROM cpu_usage` or start, and either report the row count in the results, or be more
opinionated and fail if the database is empty.
