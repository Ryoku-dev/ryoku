#!/usr/bin/env python3
"""Export the shell's config files to YAML (shell.yaml beside them)."""

import json
import os
import sys

ROOTS = ("shell.json", "shell-tokens.json")
GLOBAL_ONLY = ("keybinds.json", "cli.json")

errors = []


def load_json(path, default):
    if not os.path.exists(path):
        return default
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except Exception as e:
        errors.append(f"{path}: {e}")
        return default


_YAML_UNPRINTABLE = {c: f"\\x{c:02x}" for c in range(0x7F, 0xA0)}
_YAML_UNPRINTABLE.update({0x2028: "\\u2028", 0x2029: "\\u2029"})


def scalar(v):
    if v is None:
        return "null"
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float)):
        return str(v)
    return json.dumps(str(v), ensure_ascii=False).translate(_YAML_UNPRINTABLE)


def dump(v, ind):
    pad = "  " * ind
    if isinstance(v, dict):
        if not v:
            return "{}\n"
        out = ""
        for k, item in v.items():
            if isinstance(item, (dict, list)):
                nested = dump(item, ind + 1)
                if nested in ("[]\n", "{}\n"):
                    out += pad + scalar(k) + ": " + nested
                else:
                    out += pad + scalar(k) + ":\n" + nested
            else:
                out += pad + scalar(k) + ": " + scalar(item) + "\n"
        return out
    if isinstance(v, list):
        if not v:
            return "[]\n"
        out = ""
        for item in v:
            if isinstance(item, (dict, list)):
                out += pad + "-\n" + dump(item, ind + 1)
            else:
                out += pad + "- " + scalar(item) + "\n"
        return out
    return scalar(v)


def write_atomic(path, text):
    tmp = path + ".tmp"
    try:
        with open(tmp, "w", encoding="utf-8") as f:
            f.write(text)
        os.replace(tmp, path)
    except OSError:
        if os.path.exists(tmp):
            os.unlink(tmp)
        raise


def main():
    config_dir, state_dir = sys.argv[1:3]
    out = {}
    for file in ROOTS + GLOBAL_ONLY:
        out[os.path.splitext(file)[0]] = load_json(os.path.join(config_dir, file), {})
    out["notes"] = load_json(os.path.join(state_dir, "notes_tab.json"), [])

    monitors = {}
    mon_dir = os.path.join(config_dir, "monitors")
    if os.path.isdir(mon_dir):
        for name in sorted(os.listdir(mon_dir)):
            layers = {}
            for file in ROOTS:
                v = load_json(os.path.join(mon_dir, name, file), None)
                if v is not None:
                    layers[os.path.splitext(file)[0]] = v
            if layers:
                monitors[name] = layers
    out["monitors"] = monitors

    if errors:
        for e in errors:
            print(f"export_config: {e}", file=sys.stderr)
        sys.exit(1)
    write_atomic(os.path.join(config_dir, "shell.yaml"), dump(out, 0))
    print("DONE")


main()
