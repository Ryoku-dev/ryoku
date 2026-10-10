#!/usr/bin/env fish

set -l requested_theme $argv[1]
set -l script_dir (dirname (readlink -f (status filename)))
set -l repo_root (dirname $script_dir)
set -l build_dir "$script_dir/bibata_cursor"
set -l themes_json "$repo_root/themes.json"
set -l install_dir $BIBATA_MATERIAL_INSTALL_DIR
if test -z "$install_dir"
    set install_dir "$HOME/.icons"
end
mkdir -p $install_dir

if not command -v jq >/dev/null
    echo "Error: jq is required." >&2
    exit 1
end

if not test -f $themes_json
    echo "Error: themes.json not found at $themes_json" >&2
    exit 1
end

if not test -d $build_dir
    echo "Error: pinned bibata_cursor source is missing." >&2
    exit 1
end

set -l theme_names
if test -n "$requested_theme"
    if not jq -e --arg n "$requested_theme" 'has($n)' $themes_json >/dev/null
        echo "Error: '$requested_theme' not found in themes.json" >&2
        exit 1
    end
    set theme_names $requested_theme
else
    set theme_names (jq -r 'keys[]' $themes_json)
end

set -l fail_count 0
set -l ok_count 0
for NAME in $theme_names
    set -l BODY (jq -r --arg n "$NAME" '.[$n].body' $themes_json)
    set -l PRIMARY (jq -r --arg n "$NAME" '.[$n].primary' $themes_json)
    set -l WATCH (jq -r --arg n "$NAME" '.[$n].watch' $themes_json)
    set -l theme_name "Bibata-Material-$NAME"
    set -l temp_run_dir "$build_dir/run_$NAME"
    rm -rf $temp_run_dir
    mkdir -p $temp_run_dir
    cp -r $build_dir/src $build_dir/svg $build_dir/config $temp_run_dir/
    cd $temp_run_dir

    set -l py_status (python3 -c "
import json, sys
try:
    with open('config/render.json') as f:
        d = json.load(f)
    t = 'Bibata-Modern-Classic'
    d[t]['colors'][0]['replace'] = '$BODY'
    d[t]['colors'][1]['replace'] = '$PRIMARY'
    d[t]['colors'][2]['replace'] = '$WATCH'
    with open('config/render.json', 'w') as f:
        json.dump(d, f, indent=2)
except Exception as e:
    print(f'render.json patch failed: {e}', file=sys.stderr)
    sys.exit(1)
"; echo $status)

    if test "$py_status" != "0"
        cd $build_dir
        rm -rf $temp_run_dir
        set fail_count (math $fail_count + 1)
        continue
    end

    if not ./src/cursor_utils.py --x11 --hypr --theme "Bibata-Modern-Classic" --out-dir out
        cd $build_dir
        rm -rf $temp_run_dir
        set fail_count (math $fail_count + 1)
        continue
    end

    if test -d out/Bibata-Modern-Classic
        rm -rf "$install_dir/$theme_name"
        mv out/Bibata-Modern-Classic "$install_dir/$theme_name"
        if python3 "$script_dir/metadata_generator.py" --install-dir "$install_dir" \
                --themes-json "$themes_json" --theme "$NAME"
            set ok_count (math $ok_count + 1)
        else
            set fail_count (math $fail_count + 1)
        end
    else
        set fail_count (math $fail_count + 1)
    end
    cd $build_dir
    rm -rf $temp_run_dir
end

if test $fail_count -gt 0
    exit 1
end
