.PHONY: build test clean

build: bin
	cd plumbing/digest && go build -o ../../bin/digest

bin:
	mkdir -p bin

test:
	cd plumbing/digest && go test -race -count=3 -v ./...

clean:
	rm -rf bin/
