
all: lint test docker-run

gen:
	go generate -x ./...

lint:
	golangci-lint run

test:
	go test -race ./...

run:
	go run . data/query_params.csv

docker-build:
	docker build -t pgbench .

docker-clean:
	docker-compose down -v --rmi local

docker-run:
	docker-compose up --abort-on-container-exit

tar:
	tar --exclude=./pgbench --exclude=./.git --exclude=./.idea --transform="s|/|/${ARCHIVE_NAME}/|" -cvzf ../${ARCHIVE_NAME}.tar.gz .
