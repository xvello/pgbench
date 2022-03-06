## Questions

- Should each query be run once, or several times? -> option

## Assumptions

- Cannot use benchmarking framework, but OK to delegate work to specialised libraries
- Using local psql instance in a docker container, not TSDB Cloud
- No cache busting, input is designed to avoid hitting the cache too much

## Out of scope

- Real-time statistics via bucketing of the timings
- Bundle fixtures in the source, don't download from dropbox

## Design 
### Parsing and work assignment

- Split work with consistent hashing
with fnv: 60 52 44 44


### Execution workers

### Statistics

- Keep track of error rate too
- Median will require quantile computation -> add p90 and p95 too

---

Hello Ivan,

Thanks for being available to discuss this assignment. Before starting, I want to validate some assumptions with you.

I will implement the solution in Go, using goroutines to fan-out the work on several psql connections:
- the main goroutine will parse the CSV input data and distribute the work on multiple worker goroutines via channels,
- the worker goroutines will execute queries sequentially, reusing a prepared statement, and write to a shared output channel,
- a final goroutine will read this output channel and compute statistics that will be returned when all queries are executed.

Here are the assumptions I plan on working with, please tell me if I should reconsider any of them:

### General considerations

- I will use third-party libraries to handle non-core logic (argument parsing, DB connection, quantile computation...)
and focus on the main added value. Of course, using a fully-fledged load-testing framework like Gatling is off the table.
- Due to the requirement to set up the database on `docker-compose up`, I will use
[the official timescaledb docker container](https://hub.docker.com/r/timescale/timescaledb) instead of a Timescale Cloud
instance.

### Spreading the work

Due to the locality constraint on hostname, each worker will listen to a dedicated channel. I plan on using consistent
hashing on the hostname parameter to split the work across the worker pool. It might produce an imbalance with the
small input set I received (10 hostnames only), but would work great if the user sends a very long query set
via stdin.

If the input was guaranteed to be small, we could alternatively parse it completely then compute query sets of equal
size. Because query runtimes are not guaranteed to be similar (time range is configurable), I don't think the added
complexity is worth it for this project.

### Executing queries and measuring the performance

- Measuring the query execution time on the server side is not trivial (`EXPLAIN` will skew the measure as the
server will do extra work, and tricks using `clock_timestamp()` seem a bit brittle to me), so I plan on measuring
the wall time as seen by the client. This will include network latency, but it is negligible locally and can be
reduced when benchmarking a cloud instance, by running the CLI in the same availability-zone as the server.
Should I investigate another way to measure the query execution time?
- When benchmarking Elasticsearch queries, I had to use cache-busting techniques to ensure I was actually
hitting the Lucene indexes. I could not find any indication of result caching in Postgres, can you please
confirm to me that I didn't miss something in my investigation?
- Execution errors will not be fatal to the program, but the error rate will be tracked in the statistics.

### Computing statistics

Looking at the small size of the data set and query input, I don't see value in displaying partial statistics
to the user while the benchmark is running. A single report will be returned after all the queries have run,
but this is an improvement we could discuss for a later version.
