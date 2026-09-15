#!/usr/bin/env sh
#
# Runs the Go unit tests natively on the build host. The daemon is pure Go with
# no cgo, so this needs no cross-compiler or device.
#
#   sh tests/run.sh

set -eu

cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

if ! command -v go >/dev/null 2>&1; then
	echo "go toolchain not found" >&2
	exit 1
fi

echo "== gofmt =="
unformatted=$(gofmt -l daemon)
if [ -n "$unformatted" ]; then
	echo "not gofmt-clean:" >&2
	echo "$unformatted" >&2
	exit 1
fi

echo "== go vet (linux/arm64, the shipped target) =="
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go vet ./daemon

echo "== go test =="
CGO_ENABLED=0 go test ./daemon/

echo "== cross-compile both shipped architectures =="
# Trim tags come from the pinned tailcat module so they track upstream bumps.
tags="$(cat "$(go list -m -f '{{.Dir}}' github.com/tailscale/tailcat)/build-tags.txt"),ts_omit_ssh"
for target in "arm64:" "arm:7"; do
	arch="${target%%:*}"
	arm="${target##*:}"
	echo "-- linux/$arch${arm:+v$arm}"
	CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOARM="$arm" \
		go build -mod=readonly -trimpath -tags="$tags" -o /dev/null ./daemon
done

echo "all tests passed"
