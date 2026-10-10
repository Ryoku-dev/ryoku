#!/usr/bin/env bash
set -euo pipefail
manager=${RYOKU_TEST_DNF:-dnf}
here=$(cd "$(dirname "$0")" && pwd)
plugin=dnf-plugins-core
if [[ $(basename "$(readlink -f "$(command -v "$manager")")") == dnf5 ]]; then
  plugin=dnf5-plugins
fi
"$manager" -y install "$plugin"
while IFS= read -r repo; do
  [[ -n $repo && $repo != \#* ]] || continue
  "$manager" -y copr enable "$repo" || { echo "cannot enable required COPR $repo" >&2; exit 1; }
done < "$here/dependency-coprs"
