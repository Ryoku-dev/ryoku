#!/usr/bin/env bash
set -euo pipefail

if [[ "${INIR_SKIP_VOID_PROFILE_PREFLIGHT:-false}" == true ]]; then
  printf 'Void release-profile preflight skipped by INIR_SKIP_VOID_PROFILE_PREFLIGHT=1\n'
  exit 0
fi

arch="${INIR_TEST_VOID_ARCH:-$(uname -m)}"
if [[ "$arch" != x86_64 ]]; then
  printf 'Unsupported Void architecture: %s\n' "$arch" >&2
  printf 'This iNiR release supports Void Linux x86_64 glibc only.\n' >&2
  exit 2
fi

if [[ -n "${INIR_TEST_VOID_LIBC:-}" ]]; then
  libc="$INIR_TEST_VOID_LIBC"
else
  libc="$(getconf GNU_LIBC_VERSION 2>/dev/null || true)"
fi
if [[ "$libc" != glibc\ * ]]; then
  printf 'Unsupported Void libc: %s\n' "${libc:-unknown/musl}" >&2
  printf 'This iNiR release supports Void Linux x86_64 glibc only; musl is not a release target.\n' >&2
  exit 3
fi

printf 'Void release profile supported: %s / %s\n' "$arch" "$libc"
