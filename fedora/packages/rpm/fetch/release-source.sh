#!/usr/bin/env bash

fetch_github_release_source() {
  local repo=$1 package=$2 work=$3 sources=$4 tag_pattern=$5
  local api=${RYOKU_GITHUB_API:-https://api.github.com}
  local download=${RYOKU_GITHUB_DL:-https://github.com}
  local token=${GH_TOKEN:-${GITHUB_TOKEN:-}}
  local release tag version archive sidecar asset count expected filename extra actual
  local -a curl_auth=() lines=()

  for tool in curl jq sha256sum; do
    command -v "$tool" >/dev/null 2>&1 || {
      printf 'fetch/%s.sh: %s is required\n' "$package" "$tool" >&2
      return 1
    }
  done

  if [[ -n $token ]]; then
    [[ $token =~ ^[A-Za-z0-9._-]+$ ]] || {
      printf 'fetch/%s.sh: GitHub token contains unsupported characters\n' "$package" >&2
      return 1
    }
    (umask 077
      printf 'header = "Authorization: Bearer %s"\n' "$token" > "$work/curl-auth.conf")
    curl_auth=(--config "$work/curl-auth.conf")
  fi

  release=$(curl -fsSL --retry 3 \
    -H 'Accept: application/vnd.github+json' \
    -H 'User-Agent: ryoku-fedora-source-fetch' \
    "${curl_auth[@]}" "$api/repos/$repo/releases/latest") || {
      printf 'fetch/%s.sh: could not resolve the latest %s release\n' "$package" "$repo" >&2
      return 1
    }
  [[ $(jq -r '.draft' <<<"$release") == false ]] || {
    printf 'fetch/%s.sh: latest release is a draft\n' "$package" >&2
    return 1
  }
  [[ $(jq -r '.prerelease' <<<"$release") == false ]] || {
    printf 'fetch/%s.sh: latest release is a prerelease\n' "$package" >&2
    return 1
  }

  tag=$(jq -r '.tag_name // empty' <<<"$release")
  [[ $tag =~ $tag_pattern ]] || {
    printf "fetch/%s.sh: latest release tag '%s' is invalid\n" "$package" "$tag" >&2
    return 1
  }
  version=${tag#v}
  archive="$package-$version.tar.gz"
  sidecar="$archive.sha256"

  for asset in "$archive" "$sidecar"; do
    count=$(jq --arg name "$asset" '[.assets[] | select(.name == $name)] | length' <<<"$release")
    [[ $count == 1 ]] || {
      printf 'fetch/%s.sh: release %s must contain exactly one %s asset\n' \
        "$package" "$tag" "$asset" >&2
      return 1
    }
  done

  curl -fsSL --retry 3 -o "$work/$archive" \
    "$download/$repo/releases/download/$tag/$archive" || {
      printf 'fetch/%s.sh: could not download %s\n' "$package" "$archive" >&2
      return 1
    }
  curl -fsSL --retry 3 -o "$work/$sidecar" \
    "$download/$repo/releases/download/$tag/$sidecar" || {
      printf 'fetch/%s.sh: could not download %s\n' "$package" "$sidecar" >&2
      return 1
    }

  mapfile -t lines < <(grep -vE '^[[:space:]]*$' "$work/$sidecar" || true)
  (( ${#lines[@]} == 1 )) || {
    printf 'fetch/%s.sh: source checksum sidecar must contain one entry\n' "$package" >&2
    return 1
  }
  read -r expected filename extra <<<"${lines[0]}"
  filename=${filename#\*}
  [[ $expected =~ ^[0-9a-fA-F]{64}$ ]] || {
    printf 'fetch/%s.sh: source checksum is malformed\n' "$package" >&2
    return 1
  }
  [[ $filename == "$archive" && -z ${extra:-} ]] || {
    printf 'fetch/%s.sh: source checksum sidecar must name exactly %s\n' \
      "$package" "$archive" >&2
    return 1
  }
  actual=$(sha256sum "$work/$archive")
  actual=${actual%% *}
  [[ ${actual,,} == "${expected,,}" ]] || {
    printf 'fetch/%s.sh: sha256 mismatch for %s\n' "$package" "$archive" >&2
    return 1
  }

  mkdir -p "$sources"
  install -m0644 "$work/$archive" "$sources/$archive"
  printf '%s\n' "$version" > "$sources/.version"
  # shellcheck disable=SC2034 # read by the fetch script that sources this helper
  RYOKU_FETCH_VERSION=$version
  # shellcheck disable=SC2034
  RYOKU_FETCH_ARCHIVE=$archive
  # shellcheck disable=SC2034
  RYOKU_FETCH_ARCHIVE_PATH=$work/$archive
}
