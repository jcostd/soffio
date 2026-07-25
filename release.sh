#!/bin/sh
# release.sh - Compile fully static binaries with embedded versioning.
set -e

VERSION=$(cat VERSION)
DIST_DIR="dist"
LDFLAGS="-s -w -X main.Version=$VERSION"

rm -rf "$DIST_DIR"

echo "==> Building Soffio v$VERSION release..."

# Linux (amd64)
echo "==> Compiling for Linux (amd64)..."
TARGET="$DIST_DIR/soffio-$VERSION-linux-amd64"
mkdir -p "$TARGET"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$TARGET/soffio" ./cmd/soffio
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$TARGET/preview" ./cmd/preview

# Windows (amd64)
echo "==> Compiling for Windows (amd64)..."
TARGET="$DIST_DIR/soffio-$VERSION-windows-amd64"
mkdir -p "$TARGET"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$TARGET/soffio.exe" ./cmd/soffio
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$TARGET/preview.exe" ./cmd/preview

# macOS (Apple Silicon)
echo "==> Compiling for macOS (arm64)..."
TARGET="$DIST_DIR/soffio-$VERSION-darwin-arm64"
mkdir -p "$TARGET"
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" -o "$TARGET/soffio" ./cmd/soffio
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" -o "$TARGET/preview" ./cmd/preview

# macOS (Intel)
echo "==> Compiling for macOS (amd64)..."
TARGET="$DIST_DIR/soffio-$VERSION-darwin-amd64"
mkdir -p "$TARGET"
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$TARGET/soffio" ./cmd/soffio
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$TARGET/preview" ./cmd/preview

echo "==> Done! Release packages are available in ./$DIST_DIR/"
