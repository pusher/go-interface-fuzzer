.PHONY: all default fmt build clean

all: default

fmt:
	go fmt ./...

build: go-interface-fuzzer

clean:
	rm go-interface-fuzzer

go-interface-fuzzer: **/*.go
	cd fuzzer && go build -o ../$@
