#!/usr/bin/env bash
# Void OCR checks for the complete OCR language provider in a live Void session.
set -u

XDG_BIN_HOME="${XDG_BIN_HOME:-$HOME/.local/bin}"
export PATH="$XDG_BIN_HOME:$PATH"

failures=0
check() {
  if "$@"; then
    printf 'PASS: %s\n' "$*"
  else
    printf 'FAIL: %s\n' "$*" >&2
    failures=$((failures + 1))
  fi
}

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-pr51-ocr}"
expected_commit="${INIR_EXPECTED_COMMIT:-}"
actual_branch="$(git -C "$repo_root" branch --show-current 2>/dev/null || true)"
actual_commit="$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || true)"

check test "$actual_branch" = "$expected_branch"
if [[ -n "$expected_commit" ]]; then
  check test "$actual_commit" = "$expected_commit"
else
  printf 'INFO: commit=%s (not pinned; set INIR_EXPECTED_COMMIT to pin it)\n' "$actual_commit"
fi
if [[ "${INIR_ALLOW_DIRTY:-false}" != true ]]; then
  check test -z "$(git -C "$repo_root" status --porcelain 2>/dev/null)"
fi

deps="$repo_root/sdata/dist-void/install-deps.sh"
for package in \
  tesseract-ocr-eng \
  tesseract-ocr-spa \
  tesseract-ocr-rus \
  tesseract-ocr-jpn \
  tesseract-ocr-chi_sim \
  tesseract-ocr-chi_tra; do
  check xbps-query -p pkgver "$package"
done

for needle in \
  'TESSDATA_FAST_COMMIT="87416418657359cb625c412a48b6e1d6d41c29bd"' \
  'install_void_ocr_models' \
  'configure_void_tesseract_command' \
  'ocr-jpn-vert' \
  'ocr-chi-sim-vert' \
  'ocr-chi-tra-vert'; do
  if grep -Fq "$needle" "$deps"; then
    printf 'PASS: provider contains: %s\n' "$needle"
  else
    printf 'FAIL: provider missing: %s\n' "$needle" >&2
    failures=$((failures + 1))
  fi
done

check command -v tesseract
check tesseract --version
adapter="${XDG_BIN_HOME:-$HOME/.local/bin}/tesseract"
check test -x "$adapter"
check grep -Fq 'exec tesseract-ocr "$@"' "$adapter"

tessdata_dir="${INIR_TESSDATA_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/inir/tessdata}"
declare -A expected_hashes=(
  [jpn_vert]="bf1e2640954691797e2dc14f38533e601b59ee37958698ae0f0b81dc6f09c71b"
  [chi_sim_vert]="20590de84725bab69cde93bd6e8ed360a13cc5421a7e7364ddeb93e9af53d6da"
  [chi_tra_vert]="1df02a4b210e5c217b783819538b63e9dfe6904e2b5e53b62664f1b9f7a989d0"
)

for lang in jpn_vert chi_sim_vert chi_tra_vert; do
  model="$tessdata_dir/$lang.traineddata"
  check test -s "$model"
  check bash -c 'printf "%s  %s\n" "$1" "$2" | sha256sum -c - >/dev/null' _ "${expected_hashes[$lang]}" "$model"
done

if command -v magick >/dev/null 2>&1; then
  image="$(mktemp --suffix=.png)"
  check magick -size 64x64 xc:white "$image"
  for lang in jpn_vert chi_sim_vert chi_tra_vert; do
    check tesseract "$image" stdout --tessdata-dir "$tessdata_dir" -l "$lang" --psm 6
  done
  rm -f "$image"
else
  printf 'INFO: ImageMagick unavailable; OCR model load smoke test skipped\n'
fi

if [[ "${INIR_VERIFY_IDEMPOTENCY:-false}" == true ]]; then
  before="$(mktemp)"
  after="$(mktemp)"
  provider_snapshot() {
    for lang in jpn_vert chi_sim_vert chi_tra_vert; do
      sha256sum "$tessdata_dir/$lang.traineddata"
    done
  }
  provider_snapshot > "$before"
  check env ONLY_MISSING_DEPS='ocr-rus ocr-jpn ocr-jpn-vert ocr-chi-sim ocr-chi-sim-vert ocr-chi-tra ocr-chi-tra-vert' \
    "$repo_root/setup" install -y --skip-sysupdate --skip-setups --skip-files
  provider_snapshot > "$after"
  check cmp -s "$before" "$after"
  rm -f "$before" "$after"
else
  printf 'INFO: set INIR_VERIFY_IDEMPOTENCY=true to run the second-install snapshot check\n'
fi

if ((failures > 0)); then
  printf '%d Void OCR check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void OCR checks passed\n'
