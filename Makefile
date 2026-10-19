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
#     data (-s -w); go stamps the version from the tag, as go version -m
#     shows it;
#   - optimized by the compiler at full, as go always does, and also from
#     cmd/soffio/default.pgo when that profile is there.
#
#   make                                     soffio and preview, for here
#   make GOOS=windows GOARCH=amd64 OUT=dir   for another system, into dir
#   make test                                go vet, then the tests and their coverage

GOOS != go env GOOS
GOARCH != go env GOARCH
OUT = .

all:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	GOAMD64=v1 GOARM64=v8.0 GOARM=6 GO386=sse2 \
	go build -trimpath -ldflags '-s -w' -o $(OUT)/ ./cmd/soffio ./cmd/preview

test:
	go vet ./...
	go test -cover ./...

.PHONY: all test
