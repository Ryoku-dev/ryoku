#!/usr/bin/env bash
# Every [ryoku] package must build without resolving dependencies over the
# network. Pinned, checksummed source=() downloads are fine; unpinned dependency
# resolution is not, because it is neither verifiable nor available offline.
#
# Two regressions this pins:
#   1. limine-mkinitcpio-hook let Gradle resolve org.graalvm.buildtools.native
#      from the plugin portal. A portal hiccup failed the publish gate with
#      "Plugin [id: 'org.graalvm.buildtools.native'] was not found", so no user
#      got the update. The plugin is vendored in release/gradle-offline/repo and
#      resolved from there first. Their compile deps and the plugin's
#      reachability-metadata zip are not vendored yet, so these two builds are
#      not fully offline and --offline is deliberately not asserted.
#   2. ryoku-hub and ryoku-rashin ran a bare `go build`, which downloads modules
#      from proxy.golang.org. They build -mod=vendor from a committed vendor/.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$here/.."
pkgs="$root/release/packages"
fail() { echo "FAIL: $1" >&2; exit 1; }

# The build() body with comments stripped. Assertions must look at the commands
# that run, never at prose: an earlier version of this test matched the PKGBUILD
# comments explaining the flags and so passed even with the flags deleted.
build_body() {
  sed -n '/^build() {/,/^}/p' "$1" | sed -e 's/#.*//' -e '/^[[:space:]]*$/d'
}

# --- gradle builds resolve from the vendored maven repo, offline -------------
mapfile -t gradle_pkgs < <(grep -rl "bin/gradle" "$pkgs"/*/PKGBUILD | sort)
[[ ${#gradle_pkgs[@]} -gt 0 ]] || fail "no gradle PKGBUILDs found; did the layout change?"
for p in "${gradle_pkgs[@]}"; do
  n=$(basename "$(dirname "$p")")
  body=$(build_body "$p")
  grep -qF 'offline.init.gradle' <<<"$body" || fail "$n: gradle build must pass -I offline.init.gradle"
  grep -qF 'ryoku.offline.repo' <<<"$body" || fail "$n: gradle build must set ryoku.offline.repo"
done

# --- the shared vendored repo is present and carries the plugin -------------
go="$root/release/gradle-offline"
[[ -f $go/offline.init.gradle ]] || fail "release/gradle-offline/offline.init.gradle is missing"
[[ -f $go/refresh.sh ]] || fail "release/gradle-offline/refresh.sh is missing"
[[ -d $go/repo/org/graalvm/buildtools/native-gradle-plugin ]] \
  || fail "release/gradle-offline/repo carries no native-gradle-plugin artifacts"
count=$(find "$go/repo" -name '*.jar' | wc -l)
(( count > 0 )) || fail "release/gradle-offline/repo carries no jars"

# --- in-repo go builds use the committed vendor tree ------------------------
# The monorepo recipes build through release/packages/<pkg>/payload.sh, shared
# by the PKGBUILD and the Void template; its `build)` branch is what runs.
payload_build_body() {
  sed -n '/^build)/,/^[[:space:]]*;;/p' "$1" | sed -e 's/#.*//' -e '/^[[:space:]]*$/d'
}
# shellcheck disable=SC2016  # $RYOKU_BUILD_DIR is matched literally in the payloads
mapfile -t go_pkgs < <(grep -rlE 'go build .*-o "\$RYOKU_BUILD_DIR' "$pkgs"/*/payload.sh | sort)
[[ ${#go_pkgs[@]} -gt 0 ]] || fail "no go payload builds found; did the layout change?"
for p in "${go_pkgs[@]}"; do
  n=$(basename "$(dirname "$p")")
  grep -qE 'go build[^|&]*-mod=vendor' <<<"$(payload_build_body "$p")" \
    || fail "$n: go build must pass -mod=vendor so it never reaches proxy.golang.org"
done

# every Go module with dependencies must vendor them; a stdlib-only module needs
# nothing, since `go build` then never consults a proxy.
while IFS= read -r mod; do
  d=$(dirname "$mod")
  grep -qE '^\s*require |^require \(' "$mod" || continue
  [[ -d $d/vendor ]] \
    || fail "${d#"$root"/} declares requires but has no committed vendor/ tree, so its build needs the network"
done < <(find "$root/ryoku" -name go.mod -not -path '*/vendor/*' | sort)

echo "release-offline-builds: OK"
