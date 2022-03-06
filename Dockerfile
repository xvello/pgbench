# builder image
FROM docker.io/golang:1.17-alpine as builder
WORKDIR /build
COPY . .
RUN go build .

# runtime image
FROM docker.io/alpine
WORKDIR /
COPY --from=builder /build/pgbench .

USER 1000
CMD [ "/pgbench", "query_params.csv" ]
