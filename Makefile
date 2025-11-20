.PHONY: all clean bsky digest test

all: bsky digest

bin:
	mkdir -p bin

bsky: bin
	cd plumbing/bsky && go build -o ../../bin/bsky

digest: bin
	cd plumbing/digest && go build -o ../../bin/digest

test:
	cd plumbing/digest && go test -v ./...

clean:
	rm -rf bin/
