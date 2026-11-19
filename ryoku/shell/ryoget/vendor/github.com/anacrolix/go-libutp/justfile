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

# netgo and the suppressions file only matter on macOS; both are inert on Linux.
# See lsan_suppressions.txt.
# Leak-sanitized tests, as in the CI `asan` job.
asan:
    ASAN_OPTIONS=detect_leaks=1 LSAN_OPTIONS=suppressions={{ justfile_directory() }}/lsan_suppressions.txt go test -tags 'lsan netgo' ./...
