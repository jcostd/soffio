.PHONY: all test serve clean

all: soffio preview

soffio:
	go build ./cmd/soffio

preview:
	go build ./cmd/preview

serve: all
	./preview

test:
	go test ./...

clean:
	rm -f soffio preview
	rm -rf public
