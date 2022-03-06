#!/bin/bash

set -euox pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c \
	"\COPY cpu_usage FROM /docker-entrypoint-initdb.d/cpu_usage.csv CSV HEADER"
