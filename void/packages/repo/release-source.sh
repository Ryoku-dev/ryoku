#!/usr/bin/env bash

# latest_github_source PACKAGE REPO TAG_PATTERN WORK [RELEASE_JSON]
# RELEASE_JSON is a release the caller already resolved; without it the repo's
# GitHub "latest" release is used.
latest_github_source() {
	local package=$1 repo=$2 tag_pattern=$3 work=$4 release=${5:-}
	local api=${RYOKU_GITHUB_API:-https://api.github.com}
	local download=${RYOKU_GITHUB_DL:-https://github.com}
	local token=${GH_TOKEN:-${GITHUB_TOKEN:-}}
	local tag version archive sidecar asset count sums sum file extra
	local -a curl_auth=()

	if [[ -n $token ]]; then
		[[ $token =~ ^[A-Za-z0-9._-]+$ ]] || return 1
		(umask 077
			printf 'header = "Authorization: Bearer %s"\n' "$token" > "$work/$package-curl-auth.conf")
		curl_auth=(--config "$work/$package-curl-auth.conf")
	fi

	if [[ -z $release ]]; then
		release=$(curl -fsSL --retry 3 -H 'Accept: application/vnd.github+json' \
			-H 'User-Agent: ryoku-void-source-fetch' "${curl_auth[@]}" \
			"$api/repos/$repo/releases/latest") || return 1
	fi
	[[ $(jq -r '.draft' <<<"$release") == false && \
	   $(jq -r '.prerelease' <<<"$release") == false ]] || return 1
	tag=$(jq -r '.tag_name // empty' <<<"$release")
	[[ $tag =~ $tag_pattern ]] || return 1
	version=${tag#v}
	archive="$package-$version.tar.gz"
	sidecar="$archive.sha256"

	for asset in "$archive" "$sidecar"; do
		count=$(jq --arg name "$asset" '[.assets[] | select(.name == $name)] | length' <<<"$release")
		[[ $count == 1 ]] || return 1
	done

	sums=$(curl -fsSL --retry 3 \
		"$download/$repo/releases/download/$tag/$sidecar") || return 1
	[[ $(grep -c . <<<"$sums") == 1 ]] || return 1
	read -r sum file extra <<<"$sums"
	[[ $sum =~ ^[0-9a-fA-F]{64}$ && ${file#\*} == "$archive" && -z ${extra:-} ]] \
		|| return 1
	printf '%s %s\n' "$version" "${sum,,}"
}
