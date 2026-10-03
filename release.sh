#!/bin/bash
# Copyright (C) 2026 Jacopo Costantini
# SPDX-License-Identifier: GPL-3.0-or-later
#
# release.sh: build soffio and preview with make for every system in
# TARGETS, into dist/soffio-<VERSION>-<os>-<arch>.tar.gz, .zip for
# windows, each holding soffio-<VERSION>-<os>-<arch>/ with the two
# programs, README, LICENSE and templates/: the built-in templates, as
# files to start a -t of one's own from. How they are compiled is the
# Makefile's business. fucina-factory takes the archives as they are.

cd "$(dirname "$0")" || exit 1
CWD=$(pwd)
VERSION=$(cat VERSION)
TARGETS=${TARGETS:-"linux/386 linux/amd64 linux/arm linux/arm64 darwin/amd64 darwin/arm64 windows/386 windows/amd64 windows/arm64"}

set -eu

# the binaries carry the git revision: build only the clean tag
if [ -n "$(git status --porcelain)" ]; then
  echo "release.sh: the tree is not clean" >&2
  exit 1
fi
if [ "$(git describe --tags --exact-match 2>/dev/null)" != "v$VERSION" ]; then
  echo "release.sh: HEAD is not tagged v$VERSION" >&2
  exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

rm -rf dist
mkdir dist
for t in $TARGETS; do
  OS=${t%/*}
  ARCH=${t#*/}
  NAME=soffio-$VERSION-$OS-$ARCH

  mkdir "$TMP/$NAME"
  make -s GOOS=$OS GOARCH=$ARCH OUT="$TMP/$NAME"
  cp README LICENSE "$TMP/$NAME"
  cp -R cmd/soffio/templates "$TMP/$NAME/templates"

  cd "$TMP"
  chmod 755 $NAME $NAME/soffio* $NAME/preview* $NAME/templates
  chmod 644 $NAME/README $NAME/LICENSE $NAME/templates/*
  if [ $OS = windows ]; then
    zip -qrX "$CWD/dist/$NAME.zip" $NAME
    echo "dist/$NAME.zip"
  else
    tar --sort=name --owner=0 --group=0 -czf "$CWD/dist/$NAME.tar.gz" $NAME
    echo "dist/$NAME.tar.gz"
  fi
  cd "$CWD"
done
