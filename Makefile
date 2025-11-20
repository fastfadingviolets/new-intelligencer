.PHONY: all clean bsky

all: bsky

bin:
	mkdir -p bin

bsky: bin
	cd plumbing/bsky && go build -o ../../bin/bsky

clean:
	rm -rf bin/
