#!/usr/bin/env bash
# run the desktop's pure-JS unit tests. the *.test.mjs files that exercise the
# logic helpers behind the Quickshell surfaces (launcher fuzzy ranker, ryoshot
# coordinate/keymap/annotation libs, hub display arrangement). no Quickshell or
# display dep, so they run anywhere node is around, and unlike the advisory
# qmllint job this one is a real gate. nothing to maintain: the runner finds
# every ryoku/{shell,hub,ui,apps}/**/*.test.mjs, so a new file is picked up on landing.
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}

if ! command -v node >/dev/null 2>&1; then
  echo "::error::node is required to run the shell unit tests" >&2
  exit 1
fi

mapfile -d '' tests < <(find "$ROOT/ryoku/shell" "$ROOT/ryoku/hub" "$ROOT/ryoku/ui" "$ROOT/ryoku/apps" -name '*.test.mjs' -type f -print0 | sort -z)

if (( ${#tests[@]} == 0 )); then
  echo "::error::no unit tests found under ryoku/shell, ryoku/hub, ryoku/ui, or ryoku/apps" >&2
  exit 1
fi

failed=()
for t in "${tests[@]}"; do
  echo "== ${t#"$ROOT/"} =="
  node "$t" || failed+=("${t#"$ROOT/"}")
  echo
done

if (( ${#failed[@]} )); then
  echo "::error::shell unit tests failed:" >&2
  printf '  %s\n' "${failed[@]}" >&2
  exit 1
fi

echo "shell-unit-tests: all ${#tests[@]} test file(s) passed"
