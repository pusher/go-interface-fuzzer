.PHONY: default fmt build clean

default: fmt build

fmt:
	go fmt ./...

build: go-interface-fuzzer

clean:
	rm go-interface-fuzzer

go-interface-fuzzer: **/*.go
	cd fuzzer && go build -o ../$@
