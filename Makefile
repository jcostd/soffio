# Copyright (C) 2026 Jacopo Costantini
# SPDX-License-Identifier: GPL-3.0-or-later
#
# The one way soffio and preview are built, here and by release.sh:
#   - static, with no cgo: no libc to match, they run on any system of
#     their kind;
#   - for the baseline CPU of each family (amd64 v1, arm64 v8.0, arm 6,
#     386 with sse2), whatever the environment says, so they run on the
#     oldest machine of that family;
#   - without local paths (-trimpath), stripped of symbols and debug
#     data (-s -w), with the version in;
#   - optimized by the compiler at full, as go always does, and also from
#     cmd/soffio/default.pgo when that profile is there.
#
#   make                                     soffio and preview, for here
#   make GOOS=windows GOARCH=amd64 OUT=dir   for another system, into dir
#   make test                                go vet and the tests

VERSION != cat VERSION
GOOS != go env GOOS
GOARCH != go env GOARCH
OUT = .

all:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	GOAMD64=v1 GOARM64=v8.0 GOARM=6 GO386=sse2 \
	go build -trimpath -ldflags '-s -w -X main.Version=$(VERSION)' \
		-o $(OUT)/ ./cmd/soffio ./cmd/preview

test:
	go vet ./...
	go test ./...

.PHONY: all test
