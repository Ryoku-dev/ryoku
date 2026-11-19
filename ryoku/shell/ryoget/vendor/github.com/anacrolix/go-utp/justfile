# just defaults to `sh`, which isn't reliably on PATH on Windows CI runners;
# bash always is.
set windows-shell := ["bash", "-cu"]

default:
    @just --list

# Unit tests, as in the CI `test` job.
test:
    go test -race -count 2 ./...

# Benchmarks build/smoke, as in the CI `test` job (the '@' matches no tests).
bench:
    go test -bench . -run '@' ./...

# Unit tests with the pure Go implementation selected, as in the CI `purego` job. The interop
# tests need libutp, so they exclude themselves here and under cgo-off below.
test-purego:
    go test -race -count 2 -tags purego ./...

# purego_utp selects the same implementation as purego, for builds that only want to say it about
# µTP. Only the root package's choice of Default changes with it, so only that is worth running.
test-purego-utp:
    go test -race -count 2 -tags purego_utp .

# Everything has to build and pass with cgo off entirely, as in the CI `purego` job. -race needs
# cgo, so it can't be used here, and the libutp wrapper can't build at all.
test-nocgo:
    CGO_ENABLED=0 go test -count 2 ./...
