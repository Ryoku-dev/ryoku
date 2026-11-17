#!/usr/bin/env bash
# Print the release JSON of the newest stable Ryotunes 1.x release, by version.
# GitHub's "latest" is never used: Ryotunes 2 publishes in the same repo and
# must not reach this Ryoku line, and the old pre-reset 2.x tags stay out.
# exit 3: the repo lists no 1.x release; exit 1: the lookup failed.
# env: RYOKU_RYOTUNES_REPO, RYOKU_GITHUB_API, GH_TOKEN / GITHUB_TOKEN
set -euo pipefail

repo=${RYOKU_RYOTUNES_REPO:-ryoku-dev/ryotunes}
api=${RYOKU_GITHUB_API:-https://api.github.com}
per_page=100
max_pages=10

auth=()
token=${GH_TOKEN:-${GITHUB_TOKEN:-}}
[[ -z $token ]] || auth=(-H "Authorization: Bearer $token")

# the list is newest-created first, so the first page holding a 1.x release
# holds the newest one
for ((page = 1; page <= max_pages; page++)); do
  list=$(curl -fsSL --retry 3 -H 'Accept: application/vnd.github+json' \
    -H 'User-Agent: ryoku-ryotunes-resolve' "${auth[@]}" \
    "$api/repos/$repo/releases?per_page=$per_page&page=$page") || {
    printf 'resolve-ryotunes-release.sh: could not list %s releases\n' "$repo" >&2
    exit 1
  }
  release=$(jq -c '
    [ .[] | select(.draft == false and .prerelease == false
                   and (.tag_name | type == "string")
                   and (.tag_name | test("^v1\\.[0-9]+\\.[0-9]+$"))) ]
    | max_by(.tag_name | ltrimstr("v") | split(".") | map(tonumber)) // empty' <<<"$list") || {
    printf 'resolve-ryotunes-release.sh: unreadable release list from %s\n' "$repo" >&2
    exit 1
  }
  if [[ -n $release ]]; then
    printf '%s\n' "$release"
    exit 0
  fi
  (( $(jq 'length' <<<"$list") == per_page )) || break
done
printf 'resolve-ryotunes-release.sh: %s lists no Ryotunes 1.x release\n' "$repo" >&2
exit 3
