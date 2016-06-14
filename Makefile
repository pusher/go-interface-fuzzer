.PHONY: all default fmt build test clean

all: default

default: fmt build test

fmt:
	go fmt ./...

build: go-interface-fuzzer

test:
	go test ./...

clean:
	rm go-interface-fuzzer

go-interface-fuzzer: *.go
	go build -o $@
