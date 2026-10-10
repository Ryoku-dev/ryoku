#!/usr/bin/env bash
set -euo pipefail

die() { printf 'fedora-install: %s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'USAGE'
Usage: fedora/install.sh --base-url HTTPS_URL --gpg-key-url HTTPS_URL \
  --gpg-fingerprint FINGERPRINT --provider hyprland|niri

Environment equivalents:
  RYOKU_RPM_BASE_URL
  RYOKU_RPM_GPG_KEY_URL
  RYOKU_RPM_GPG_FINGERPRINT
  RYOKU_FEDORA_PROVIDER

The repository must publish signed Fedora RPM metadata and packages under the
Ryoku DNF repository layout. The signing key fingerprint is verified before import.
USAGE
}

base_url=${RYOKU_RPM_BASE_URL:-}
gpg_key_url=${RYOKU_RPM_GPG_KEY_URL:-}
gpg_fingerprint=${RYOKU_RPM_GPG_FINGERPRINT:-}
provider=${RYOKU_FEDORA_PROVIDER:-}
while (($#)); do
  case "$1" in
    --base-url) (($# >= 2)) || die '--base-url requires a URL'; base_url=$2; shift 2 ;;
    --gpg-key-url) (($# >= 2)) || die '--gpg-key-url requires a URL'; gpg_key_url=$2; shift 2 ;;
    --gpg-fingerprint) (($# >= 2)) || die '--gpg-fingerprint requires a value'; gpg_fingerprint=$2; shift 2 ;;
    --provider) (($# >= 2)) || die '--provider requires a value'; provider=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -n $base_url && -n $gpg_key_url && -n $gpg_fingerprint ]] || { usage >&2; die 'set the repository URL, key URL, and full key fingerprint'; }
[[ $base_url =~ ^https://[^[:space:]]+$ && $gpg_key_url =~ ^https://[^[:space:]]+$ ]] || die 'repository and key URLs must be whitespace-free HTTPS URLs'
[[ $gpg_fingerprint =~ ^([[:xdigit:]]{40}|[[:xdigit:]]{64})$ ]] || die 'provide the full 40- or 64-digit OpenPGP fingerprint'
[[ $provider == hyprland || $provider == niri ]] || die 'provider must be hyprland or niri'
base_url=${base_url%/}

[[ $(id -u) -ne 0 ]] || die 'run as a normal user; sudo is used for system changes'
[[ -r /etc/os-release ]] || die 'cannot identify this operating system'
# shellcheck source=/dev/null
. /etc/os-release
[[ ${ID:-} == fedora || " ${ID_LIKE:-} " == *' fedora '* ]] || die 'this installer supports Fedora and Fedora derivatives only'
[[ $(uname -m) == x86_64 ]] || die 'Ryoku RPMs currently target x86_64'
[[ -d /run/systemd/system ]] || die 'this installer requires a mutable systemd installation'
[[ ! -e /run/ostree-booted ]] || die 'rpm-ostree systems are not supported'
command -v sudo >/dev/null 2>&1 || die 'sudo is required'
command -v curl >/dev/null 2>&1 || die 'curl is required'
command -v gpg >/dev/null 2>&1 || die 'gpg is required to verify the repository signing key; install gnupg2 first'
dnf_cmd=dnf
if command -v dnf5 >/dev/null 2>&1; then dnf_cmd=dnf5; fi
sudo -v

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -fL --retry 3 --output "$work/RPM-GPG-KEY-ryoku" "$gpg_key_url"
[[ -s "$work/RPM-GPG-KEY-ryoku" ]] || die 'the signing key download was empty'
actual_fingerprint=$(gpg --batch --with-colons --show-keys "$work/RPM-GPG-KEY-ryoku" | awk -F: '$1 == "fpr" { print $10; exit }')
[[ ${actual_fingerprint^^} == ${gpg_fingerprint^^} ]] || die 'the downloaded signing key does not match the supplied fingerprint'
cat > "$work/ryoku.repo" <<REPO
[ryoku]
name=Ryoku
baseurl=$base_url
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-ryoku
skip_if_unavailable=0
REPO

sudo install -D -m 0644 "$work/RPM-GPG-KEY-ryoku" /etc/pki/rpm-gpg/RPM-GPG-KEY-ryoku
sudo rpm --import /etc/pki/rpm-gpg/RPM-GPG-KEY-ryoku
sudo install -D -m 0644 "$work/ryoku.repo" /etc/yum.repos.d/ryoku.repo
sudo "$dnf_cmd" makecache --refresh --repo=ryoku
sudo "$dnf_cmd" install -y --repo=ryoku ryoku ryoku-desktop "ryoku-desktop-$provider"
