CGO_ENABLED := 0
LDFLAGS     := -ldflags "-s -w"
BUILDFLAGS  := -trimpath $(LDFLAGS)

.PHONY: all serve clean test

all: soffio preview

soffio:
	go build $(BUILDFLAGS) -o $@ ./cmd/soffio

preview:
	go build $(BUILDFLAGS) -o $@ ./cmd/preview

public: soffio
	./soffio -in content -out $@

serve: preview public
	./preview

test:
	go test -race ./...

clean:
	rm -rf soffio preview public
