#!/usr/bin/env bash
# emit the enabled plugin set as one JSON array on stdout. each element merges
# a manifest.json with the user's placement from plugins.json. Installed-tree
# entries also require their Store receipt, whose version is authoritative:
#   { "id", "dir", "version", "manifest": {...}, "placement": {...} }
# sources, first wins on duplicate id:
#   $RYOSTORE_PLUGINS_DIR (dev override, colon-separated; legacy RYOKU_PLUGINS_DIR still read)
#   ~/.local/share/ryoku/plugins (receipt-owned Store products only)
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
dirs+=("$installed_root")

for d in "${dirs[@]}"; do
	[ -d "$d" ] || continue
	for m in "$d"/*/manifest.json; do
		[ -f "$m" ] || continue
		manifest_files+=("$m")
		manifest_dirs+=("${m%/manifest.json}")
		if [ "$d" = "$installed_root" ]; then
			manifest_pinned+=(1)
		else
			manifest_pinned+=(0)
		fi
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
receipt_files=()
for f in "$state_root"/plugins/*.json; do
	[ -f "$f" ] || continue
	receipt_files+=("$f")
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
printf -v receipt_paths_raw '%s'"$sep" "${receipt_files[@]}"

jq -c -n "${rawfiles[@]}" \
	--argjson all "$all" \
	--arg installed_root "$installed_root" \
	--arg state_root "$state_root" \
	--rawfile index_text "$index_src" \
	--rawfile user_text "$user_src" \
	--arg manifest_dirs "$manifest_dirs_raw" \
	--arg manifest_pinned "$manifest_pinned_raw" \
	--arg view_paths "$view_paths_raw" \
	--arg receipt_paths "$receipt_paths_raw" \
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
	def store_entry($id; $user; $receipts; $index; $views; $state_root):
		at($receipts; $id) as $receipt
		| if ($receipt | type) != "object"
				or $receipt.category != "plugins"
				or $receipt.destination != ("ryoku/plugins/" + $id) then null
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
	| parts("receipt_paths") as $rpaths
	| (reduce range(0; $vpaths | length) as $i ({};
			.[$vpaths[$i]] = parsed("v\($i)"))) as $views
	| (reduce range(0; $rpaths | length) as $i ({};
			.[($rpaths[$i] | split("/") | last | sub("\\.json$"; ""))] = parsed("r\($i)"))) as $receipts
	# walk the candidates in precedence order and keep the first entry per id,
	# enabled or not: a disabled dev-override copy still shadows the installed
	# one behind it, exactly as the shell would have resolved it.
	| reduce range(0; $pdirs | length) as $i (
		{ seen: {}, out: [] };
		(obj(parsed("m\($i)"))) as $man
		| (str($man.id)) as $id
		| if ($id | test("^[a-z0-9][a-z0-9-]*$") | not) or (.seen[$id] == true) then .
			else
				.seen[$id] = true
				| if ($all == 1 or on($user; $id)) | not then .
					else
						(if ($pinned[$i] // "0") == "1"
							then store_entry($id; $user; $receipts; $index; $views; $state_root)
							else { id: $id, dir: $pdirs[$i], version: str($man.version), manifest: $man,
								placement: (at($user; $id) // {}) }
							end) as $entry
						| if $entry == null then . else .out += [$entry] end
					end
			end
	)
	| .out
'
