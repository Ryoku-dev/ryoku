#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-only
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd -P)
BUILD_DIR=${HYAKKI_BUILD_DIR:-"$ROOT/build/native"}
VERSION=${HYAKKI_VERSION:-dev}
case "$BUILD_DIR" in /*) ;; *) printf 'HYAKKI_BUILD_DIR must be absolute\n' >&2; exit 2 ;; esac
case "$VERSION" in ''|*[!A-Za-z0-9._+-]*) printf 'invalid HYAKKI_VERSION\n' >&2; exit 2 ;; esac
export GOWORK=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOENV=off GOFLAGS=
unset GOOS GOARCH
python3 "$ROOT/release/native.py" check-source
mkdir -p "$BUILD_DIR"
cd "$ROOT"
CGO_ENABLED=0 go build -mod=vendor -buildvcs=false -trimpath \
    -ldflags "-s -w -X main.version=$VERSION" -o "$BUILD_DIR/hyakki" ./cmd/hyakki
CGO_ENABLED=0 go build -mod=vendor -buildvcs=false -trimpath \
    -ldflags "-s -w -X main.version=$VERSION" -o "$BUILD_DIR/oni" ./cmd/oni
CGO_ENABLED=1 go build -mod=vendor -buildvcs=false -trimpath -tags "${HYAKKI_GIO_TAGS:-}" \
    -ldflags "-s -w -X main.version=$VERSION" -o "$BUILD_DIR/hyakki-app" ./cmd/hyakki-app
CGO_ENABLED=1 go build -mod=vendor -buildvcs=false -trimpath \
    -ldflags "-s -w -X main.version=$VERSION" -o "$BUILD_DIR/oni-surface" ./cmd/oni-surface
python3 "$ROOT/release/native.py" notices
printf 'Built Hyakki %s in %s\n' "$VERSION" "$BUILD_DIR"
