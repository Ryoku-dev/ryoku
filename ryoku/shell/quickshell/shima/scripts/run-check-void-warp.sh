#!/usr/bin/env bash
# Run the Void WARP optional-provider contract. Live WARP verification remains
# opt-in; this wrapper does not register or connect an account by itself.
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
branch="${INIR_EXPECTED_BRANCH:-$(git -C "$repo_root" branch --show-current)}"

exec env \
  INIR_EXPECTED_BRANCH="$branch" \
  INIR_EXPECTED_COMMIT="${INIR_EXPECTED_COMMIT:-$(git -C "$repo_root" rev-parse HEAD)}" \
  INIR_VERIFY_WARP_LIVE="${INIR_VERIFY_WARP_LIVE:-false}" \
  INIR_VERIFY_IDEMPOTENCY="${INIR_VERIFY_IDEMPOTENCY:-false}" \
  "$repo_root/scripts/check-void-warp.sh"
