.PHONY: build test install

build:
	go build -o dev ./cmd/dev

test:
	go test ./...

install:
	go install ./cmd/dev
