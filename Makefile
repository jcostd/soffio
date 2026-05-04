.PHONY: all build generate serve clean

# Default target
all: build

# Compile binaries
build:
	@echo "build: cmd/soffio"
	@go build -o soffio ./cmd/soffio
	@echo "build: cmd/preview"
	@go build -o preview ./cmd/preview

# Generate HTML from corpus
generate: build
	@echo "generate: building public site"
	@mkdir -p public
	@./soffio -in content -out public

# Generate site and start local server
serve: generate
	@echo "serve: starting preview server"
	@./preview

# Remove binaries and generated output
clean:
	@echo "clean: removing artifacts"
	@rm -f soffio preview
	@rm -rf public/*
