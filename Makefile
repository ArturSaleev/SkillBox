.PHONY: dashboard build benchmark build-all test fmt run run-benchmark

dashboard:
	./build-dashboard.sh

build: dashboard
	GOWORK=off go build -trimpath -o skillbox ./cmd/skillbox

benchmark:
	GOWORK=off go build -trimpath -o skillbox-bench ./benchmark/cmd/skillbox-bench

build-all: build benchmark

test:
	GOWORK=off go test ./...

fmt:
	gofmt -w cmd internal tests benchmark/cmd benchmark/internal

run: build
	./skillbox

run-benchmark: benchmark
	./skillbox-bench -config ./benchmark/config.yaml
