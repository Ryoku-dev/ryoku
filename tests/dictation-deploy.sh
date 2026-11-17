#!/usr/bin/env bash
# deploy.sh must install the Vulkan build dependencies when only SPIRV-Headers is missing.
# Every other tool is stubbed as present, so the only reason to install is the CMake package.
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
DEPLOY="$ROOT/ryoku/dictation/deploy.sh"

fail() { echo "dictation-deploy: $*" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

stub() {
	local name=$1 body=$2
	printf '#!/bin/sh\n%s\n' "$body" > "$work/bin/$name"
	chmod +x "$work/bin/$name"
}

# $1 = pkgmgr, $2 = whether SPIRV-Headers resolves (yes|no); prints what sudo was asked to run
run_deploy() {
	rm -rf "${work:?}/bin" "${work:?}/sudo.log"
	mkdir -p "$work/bin"
	stub cmake "case \"\$*\" in *SPIRV-Headers*) [ '$2' = yes ] ;; *) exit 1 ;; esac"
	for tool in ninja c++ glslc pkg-config; do stub "$tool" 'exit 0'; done
	stub ryoku-host "case \"\$1\" in init) echo systemd ;; pkgmgr) echo $1 ;; *) exit 0 ;; esac"
	stub sudo "echo \"\$*\" >> '$work/sudo.log'; exit 99"
	PATH="$work/bin:/usr/bin:/bin" bash "$DEPLOY" --no-reload >/dev/null 2>&1 || true
	cat "$work/sudo.log" 2>/dev/null || true
}

for case_ in pacman:spirv-headers xbps:SPIRV-Headers dnf:spirv-headers-devel; do
	mgr=${case_%%:*} want=${case_#*:}
	got=$(run_deploy "$mgr" no)
	[[ $got == *" $want"* || $got == *" $want" ]] \
		|| fail "$mgr: a host without SPIRV-Headers must have $want installed, ran: ${got:-nothing}"
done

got=$(run_deploy pacman yes)
[[ -z $got ]] || fail "SPIRV-Headers is present, nothing should be installed, ran: $got"

echo "dictation-deploy: ok"
