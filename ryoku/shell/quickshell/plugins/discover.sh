#!/usr/bin/env bash
# emit the enabled plugin set as one JSON array on stdout. each element merges
# a manifest.json with the user's placement from plugins.json. Installed-tree
# entries also require their Store receipt, whose version is authoritative:
#   { "id", "dir", "version", "manifest": {...}, "placement": {...} }
# sources, first wins on duplicate id:
#   $RYOSTORE_PLUGINS_DIR (dev override, colon-separated; legacy RYOKU_PLUGINS_DIR still read)
#   ~/.local/share/ryoku/plugins (receipt-owned Store products only)
#   ~/.local/share/ryoku/sources/<store>/plugins (receipt-owned, from stores the user added)
# placement + per-plugin settings live in ~/.config/ryoku/plugins.json:
#   { "<id>": { "enabled": bool, "host": "...", "<host>": {...}, "key": "...",
#               "settings": {...} } }
# no entry or enabled=false = skipped. the shell only loads what the user
# actually turned on.
#
# one pass, one jq. this script runs on every placement write -- every time a
# desktop widget is dragged, resized or recoloured -- and a per-plugin jq call
# was ~10 process spawns each, close to a second of CPU on a full desktop. the
# shell forks that in the middle of a drag and the whole desktop janks. so bash
# only globs the file sets here and hands every manifest, view and receipt to a
# single jq by name; the merge, the receipt/view chain and the dedupe all
# happen there.
set -euo pipefail

cfg_home="${XDG_CONFIG_HOME:-$HOME/.config}"
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
state_home="${XDG_STATE_HOME:-$HOME/.local/state}"
state_root="$state_home/ryoku/store"
installed_root="$data_home/ryoku/plugins"

# --all = every installed plugin (Settings wants that). default = only enabled
# (the runtime wants that).
all=0
[ "${1:-}" = "--all" ] && all=1

shopt -s nullglob

# candidate manifests in precedence order, each with the plugin dir it lives in
# and whether it came from the installed root. that distinction is the whole
# difference: an installed product is only real once its receipt, its pinned
# content view and its index row all agree, and its version comes from the
# receipt, not the manifest. a dev override dir needs none of that.
manifest_files=()
manifest_dirs=()
manifest_pinned=()
dirs=()
plugins_dir="${RYOSTORE_PLUGINS_DIR:-${RYOKU_PLUGINS_DIR:-}}"
if [ -n "$plugins_dir" ]; then
	IFS=':' read -r -a extra <<<"$plugins_dir"
	dirs+=("${extra[@]}")
fi
pins=()
for d in "${dirs[@]}"; do pins+=(0); done
dirs+=("$installed_root")
pins+=(1)
# ryostore installs one copy of an id at a time, from whichever store, so a
# user store's copy never meets an official one here
for d in "$data_home"/ryoku/sources/*/plugins; do
	dirs+=("$d")
	src="${d%/plugins}"
	pins+=("s:${src##*/}")
done

for k in "${!dirs[@]}"; do
	d="${dirs[$k]}"
	[ -d "$d" ] || continue
	for m in "$d"/*/manifest.json; do
		[ -f "$m" ] || continue
		manifest_files+=("$m")
		manifest_dirs+=("${m%/manifest.json}")
		manifest_pinned+=("${pins[$k]}")
	done
done

# the pinned content views and the receipts to resolve them against. without
# --slurpfile (which aborts the whole run on one bad file) these go in as raw
# text and parse tolerantly, so a single broken file can never blank the
# listing the desktop is about to load.
view_files=()
for f in "$state_root"/plugin-views/*/*/manifest.json; do
	[ -f "$f" ] || continue
	view_files+=("$f")
done
# receipts go in keyed <id> for the official stores, <store>/<id> otherwise
receipt_files=()
receipt_keys=()
for f in "$state_root"/plugins/*.json; do
	[ -f "$f" ] || continue
	receipt_files+=("$f")
	key="${f##*/}"
	receipt_keys+=("${key%.json}")
done
for f in "$state_root"/sources/*/plugins/*.json; do
	[ -f "$f" ] || continue
	receipt_files+=("$f")
	key="${f#"$state_root"/sources/}"
	key="${key%.json}"
	receipt_keys+=("${key%%/*}/${key##*/}")
done

rawfiles=()
i=0
for f in "${manifest_files[@]}"; do
	rawfiles+=(--rawfile "m$i" "$f")
	i=$((i + 1))
done
i=0
for f in "${view_files[@]}"; do
	rawfiles+=(--rawfile "v$i" "$f")
	i=$((i + 1))
done
i=0
for f in "${receipt_files[@]}"; do
	rawfiles+=(--rawfile "r$i" "$f")
	i=$((i + 1))
done

# missing, empty or unreadable reads as empty text and parses to nothing.
index_src=/dev/null
if [ -s "$state_root/plugins.json" ] && [ -r "$state_root/plugins.json" ]; then
	index_src="$state_root/plugins.json"
fi
user_src=/dev/null
if [ -s "$cfg_home/ryoku/plugins.json" ] && [ -r "$cfg_home/ryoku/plugins.json" ]; then
	user_src="$cfg_home/ryoku/plugins.json"
fi

# path lists travel as one separator-joined string: the record separator cannot
# appear in a real path, so the pairing survives spaces and newlines alike.
sep=$'\x1e'
printf -v manifest_dirs_raw '%s'"$sep" "${manifest_dirs[@]}"
printf -v manifest_pinned_raw '%s'"$sep" "${manifest_pinned[@]}"
printf -v view_paths_raw '%s'"$sep" "${view_files[@]}"
printf -v receipt_keys_raw '%s'"$sep" "${receipt_keys[@]}"

jq -c -n "${rawfiles[@]}" \
	--argjson all "$all" \
	--arg installed_root "$installed_root" \
	--arg state_root "$state_root" \
	--rawfile index_text "$index_src" \
	--rawfile user_text "$user_src" \
	--arg manifest_dirs "$manifest_dirs_raw" \
	--arg manifest_pinned "$manifest_pinned_raw" \
	--arg view_paths "$view_paths_raw" \
	--arg receipt_keys "$receipt_keys_raw" \
	'
	def text($k): $ARGS.named[$k] // "";
	def parsed($k): (text($k) | try fromjson catch null);
	def parts($k): (text($k) | split("\u001e") | map(select(length > 0)));
	def obj($v): if ($v | type) == "object" then $v else {} end;
	def str($v): ($v // "") | tostring;
	def at($o; $k): (if ($o | type) == "object" then ($o[$k] // null) else null end);

	# a placement is only on when it says so: anything else reads as off, the
	# way a lookup on a malformed placement would have.
	def on($user; $id): (str(at($user; $id) | if type == "object" then .enabled else null end)) == "true";

	# an installed product is real only when its receipt, its index row and the
	# pinned content view all agree, and its version comes from the receipt and
	# not from the manifest. any missing link = not installed, not an error.
	# $src is the store the copy came from, "" for the official ones: its
	# receipt has to name that store and the folder of that store.
	def store_entry($id; $src; $user; $receipts; $index; $views; $state_root):
		at($receipts; (if $src == "" then $id else $src + "/" + $id end)) as $receipt
		| if ($receipt | type) != "object"
				or $receipt.category != "plugins"
				or str($receipt.source) != $src
				or $receipt.destination != (if $src == "" then "ryoku/plugins/" + $id
					else "ryoku/sources/" + $src + "/plugins/" + $id end) then null
			else (str($receipt.version)) as $version
			| if ($version | length) == 0 then null
				else ($index | map(select(.id == $id and .version == $version)) | .[0]) as $row
				| if ($row | type) != "object" then null
					else (str($row.view)) as $view
					| if ($view | test("^plugin-views/" + $id + "/[a-f0-9]{64}$") | not) then null
						else ($state_root + "/" + $view) as $vp
						| (at($views; $vp + "/manifest.json")) as $vm
						| if ($vm | type) != "object" or (str($vm.id)) != $id then null
							else { id: $id, dir: $vp, version: $version, manifest: $vm, placement: (at($user; $id) // {}) }
							end
						end
					end
				end
			end;

	# the Store index and the user placement both read tolerantly: a bad or
	# missing file degrades to an empty one instead of failing the run.
	(parsed("index_text")) as $index_raw
	| (if ($index_raw | type) == "array" then $index_raw else [] end) as $index
	| (obj(parsed("user_text"))) as $user
	| parts("manifest_dirs") as $pdirs
	| parts("manifest_pinned") as $pinned
	| parts("view_paths") as $vpaths
	| parts("receipt_keys") as $rkeys
	| (reduce range(0; $vpaths | length) as $i ({};
			.[$vpaths[$i]] = parsed("v\($i)"))) as $views
	| (reduce range(0; $rkeys | length) as $i ({};
			.[$rkeys[$i]] = parsed("r\($i)"))) as $receipts
	# walk the candidates in precedence order and keep the first entry per id,
	# enabled or not: a disabled dev-override copy still shadows the installed
	# one behind it, exactly as the shell would have resolved it. an installed
	# copy whose receipt chain does not hold claims nothing, so it never hides
	# the copy another store really installed.
	| reduce range(0; $pdirs | length) as $i (
		{ seen: {}, out: [] };
		(obj(parsed("m\($i)"))) as $man
		| (str($man.id)) as $id
		| ($pinned[$i] // "0") as $pin
		| if ($id | test("^[a-z0-9][a-z0-9-]*$") | not) or (.seen[$id] == true) then .
			elif ($all == 1 or on($user; $id)) | not then .seen[$id] = true
			elif $pin == "0" then
				.seen[$id] = true
				| .out += [{ id: $id, dir: $pdirs[$i], version: str($man.version), manifest: $man,
					placement: (at($user; $id) // {}) }]
			else
				store_entry($id; (if $pin == "1" then "" else ($pin | ltrimstr("s:")) end);
					$user; $receipts; $index; $views; $state_root) as $entry
				| if $entry == null then . else .seen[$id] = true | .out += [$entry] end
			end
	)
	| .out
'
