#!/usr/bin/env bash
# Refresh one independently released app in a mutable pacman channel.
set -euo pipefail
umask 077

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
: "${R2_BUCKET:?}" "${GPG_PRIVATE_KEY:?}" "${RYOKU_CHANNEL:?}" "${RYOKU_APP:?}"
case "$RYOKU_APP" in
  ryotunes) display=Ryotunes ;;
  ryoport) display=Ryoport ;;
  *) echo "Unsupported independent app: $RYOKU_APP" >&2; exit 1 ;;
esac
case "$RYOKU_CHANNEL" in
  stable) remote="Ryoku:$R2_BUCKET/x86_64" ;;
  testing) remote="Ryoku:$R2_BUCKET/channels/testing/x86_64" ;;
  *) echo "Unsupported mutable channel: $RYOKU_CHANNEL" >&2; exit 1 ;;
esac
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
export GNUPGHOME="$work/keyring"
mkdir -m 700 "$GNUPGHOME" "$work/repo" "$work/db" "$work/package" "$work/metadata"
printf 'pinentry-mode loopback\n' > "$GNUPGHOME/gpg.conf"
printf '%s\n' "$GPG_PRIVATE_KEY" | gpg --batch --import
unset GPG_PRIVATE_KEY
"$root/bin/ryoku-r2-config"

rclone copyto "$remote/ryoku.db" "$work/repo/ryoku.db.tar.gz"
rclone copyto "$remote/ryoku.db.sig" "$work/repo/ryoku.db.tar.gz.sig"
gpg --batch --verify "$work/repo/ryoku.db.tar.gz.sig" "$work/repo/ryoku.db.tar.gz"
bsdtar -xf "$work/repo/ryoku.db.tar.gz" -C "$work/db"
installed=''
for desc in "$work/db"/*/desc; do
  name=$(awk '/^%NAME%$/{getline;print;exit}' "$desc")
  if [[ "$name" == "$RYOKU_APP" ]]; then
    installed=$(awk '/^%VERSION%$/{getline;print;exit}' "$desc")
    break
  fi
done

latest=$(RYOKU_IMPORT_RESOLVE_ONLY=1 RYOKU_APP="$RYOKU_APP" bash "$root/release/repo/import-ryotunes.sh")
if [[ -n "$installed" ]] && (( $(vercmp "$installed" "$latest") >= 0 )); then
  echo "$RYOKU_CHANNEL already serves $display $installed; no package download or publish needed."
  exit 0
fi

package=$(RYOKU_APP="$RYOKU_APP" bash "$root/release/repo/import-ryotunes.sh" "$work/package")
[[ -f "$package" ]] || { echo 'Importer did not return a package file' >&2; exit 1; }
version=$(bsdtar -xOf "$package" .PKGINFO | awk -F ' = ' '$1=="pkgver" {print $2;exit}')
if [[ -n "$installed" ]] && (( $(vercmp "$version" "$installed") <= 0 )); then
  echo "Refusing to replace $installed with $version" >&2
  exit 1
fi

rclone copyto "$remote/ryoku.files" "$work/repo/ryoku.files.tar.gz"
rclone copyto "$remote/ryoku.files.sig" "$work/repo/ryoku.files.tar.gz.sig"
gpg --batch --verify "$work/repo/ryoku.files.tar.gz.sig" "$work/repo/ryoku.files.tar.gz"
gpg --batch --yes --detach-sign "$package"
repo-add --sign "$work/repo/ryoku.db.tar.gz" "$package"
gpg --batch --verify "$work/repo/ryoku.db.tar.gz.sig" "$work/repo/ryoku.db.tar.gz"
gpg --batch --verify "$work/repo/ryoku.files.tar.gz.sig" "$work/repo/ryoku.files.tar.gz"

filename=$(basename -- "$package")
rclone copyto "$package" "$remote/$filename"
rclone copyto "$package.sig" "$remote/$filename.sig"
expected_size=$(stat -c %s "$package")
rclone size "$remote/$filename" --json | jq -e --argjson size "$expected_size" '.count == 1 and .bytes == $size' >/dev/null
for db in ryoku.db ryoku.files; do
  for suffix in '' '.sig'; do
    cp -- "$work/repo/$db.tar.gz$suffix" "$work/metadata/$db.tar.gz$suffix"
    cp -- "$work/repo/$db.tar.gz$suffix" "$work/metadata/$db$suffix"
  done
done
rclone copy "$work/metadata" "$remote/"
rclone copyto "$remote/ryoku.db" "$work/published.db"
rclone copyto "$remote/ryoku.db.sig" "$work/published.db.sig"
cmp -- "$work/repo/ryoku.db.tar.gz" "$work/published.db"
gpg --batch --verify "$work/published.db.sig" "$work/published.db"
echo "Published official $display $version to $RYOKU_CHANNEL; frozen releases and all other packages are unchanged."
