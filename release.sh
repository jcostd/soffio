#!/bin/bash
# Copyright (C) 2026 Jacopo Costantini
# SPDX-License-Identifier: GPL-3.0-or-later
#
# release.sh: build soffio and preview for every system in TARGETS into
# dist/soffio-<VERSION>-<os>-<arch>.tar.gz, .zip for windows, each
# holding soffio-<VERSION>-<os>-<arch>/ with the two programs, README and
# LICENSE. fucina-factory takes them as they are.

cd "$(dirname "$0")" || exit 1
CWD=$(pwd)
VERSION=$(cat VERSION)
TARGETS=${TARGETS:-"linux/386 linux/amd64 linux/arm linux/arm64 darwin/amd64 darwin/arm64 windows/386 windows/amd64 windows/arm64"}

set -eu

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

rm -rf dist
mkdir dist
for t in $TARGETS; do
  OS=${t%/*}
  ARCH=${t#*/}
  NAME=soffio-$VERSION-$OS-$ARCH
  EXE=
  [ $OS != windows ] || EXE=.exe

  mkdir "$TMP/$NAME"
  for c in soffio preview; do
    CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -trimpath \
      -ldflags "-s -w -X main.Version=$VERSION" -o "$TMP/$NAME/$c$EXE" ./cmd/$c
  done
  cp README LICENSE "$TMP/$NAME"

  cd "$TMP"
  chmod 755 $NAME $NAME/soffio$EXE $NAME/preview$EXE
  chmod 644 $NAME/README $NAME/LICENSE
  if [ $OS = windows ]; then
    zip -qrX "$CWD/dist/$NAME.zip" $NAME
    echo "dist/$NAME.zip"
  else
    tar --sort=name --owner=0 --group=0 -czf "$CWD/dist/$NAME.tar.gz" $NAME
    echo "dist/$NAME.tar.gz"
  fi
  cd "$CWD"
done
