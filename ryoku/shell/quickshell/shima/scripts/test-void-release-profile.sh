#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
probe="$repo_root/scripts/check-void-release-profile.sh"

INIR_TEST_VOID_ARCH=x86_64 INIR_TEST_VOID_LIBC='glibc 2.41' bash "$probe" >/dev/null

if INIR_TEST_VOID_ARCH=aarch64 INIR_TEST_VOID_LIBC='glibc 2.41' bash "$probe" >/tmp/inir-void-arch.out 2>&1; then
  printf 'FAIL: non-x86_64 Void profile was accepted\n' >&2
  exit 1
fi
grep -Fq 'x86_64 glibc only' /tmp/inir-void-arch.out

if INIR_TEST_VOID_ARCH=x86_64 INIR_TEST_VOID_LIBC='musl libc (x86_64)' bash "$probe" >/tmp/inir-void-musl.out 2>&1; then
  printf 'FAIL: Void musl profile was accepted\n' >&2
  exit 1
fi
grep -Fq 'musl is not a release target' /tmp/inir-void-musl.out

INIR_SKIP_VOID_PROFILE_PREFLIGHT=true INIR_TEST_VOID_ARCH=aarch64 INIR_TEST_VOID_LIBC='musl' bash "$probe" >/dev/null
printf 'Void release-profile checks passed\n'
