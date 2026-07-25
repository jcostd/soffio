#!/bin/sh
# release.sh - Compile static binaries and pack them into release archives.
set -e

VERSION=$(cat VERSION)
DIST_DIR="dist"
LDFLAGS="-s -w -X main.Version=$VERSION"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo "==> Building Soffio v$VERSION release archives..."

build() {
    os=$1
    arch=$2
    ext=""
    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi

    name="soffio-$VERSION-$os-$arch"
    target="$DIST_DIR/$name"
    mkdir -p "$target"

    echo "==> Compiling for $os ($arch)..."
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "$LDFLAGS" -o "$target/soffio$ext" ./cmd/soffio
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "$LDFLAGS" -o "$target/preview$ext" ./cmd/preview

    cp -f README.org LICENSE "$target/" 2>/dev/null || true

    if [ "$os" = "windows" ] && command -v zip >/dev/null 2>&1; then
        (cd "$DIST_DIR" && zip -q -r "$name.zip" "$name")
    else
        tar -czf "$DIST_DIR/$name.tar.gz" -C "$DIST_DIR" "$name"
    fi

    rm -rf "$target"
}

build linux amd64
build windows amd64
build darwin arm64
build darwin amd64

echo "==> Done! Release archives available in ./$DIST_DIR/:"
