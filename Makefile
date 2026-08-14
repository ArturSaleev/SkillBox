.PHONY: build test fmt run

build:
	GOWORK=off go build -trimpath -o skillbox ./cmd/skillbox

test:
	GOWORK=off go test ./...

fmt:
	gofmt -w cmd internal tests

run: build
	./skillbox
