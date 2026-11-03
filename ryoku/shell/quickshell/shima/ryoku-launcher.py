import json
import os
from pathlib import Path
import sys

def prepare(config_home, state_home, cache_home):
    config = Path(config_home) / "ryoku/shima"
    state = Path(state_home) / "ryoku/shima"
    cache = Path(cache_home) / "ryoku/shima"
    for path in (config, state, cache):
        if path.is_symlink() or str(path.resolve()).startswith("/nix/store/"):
            raise RuntimeError("Shima state must be user-owned and writable")
        path.mkdir(parents=True, exist_ok=True, mode=0o700)
    file = config / "config.json"
    if file.is_symlink():
        raise RuntimeError("Refusing to replace a managed Shima config symlink")
    if not file.exists():
        initial = {}
        legacy = Path(config_home) / "ryoku/shell.json"
        if legacy.is_file():
            value = json.loads(legacy.read_text()).get("inir", {})
            if isinstance(value, dict):
                initial = value
        initial.setdefault("panelFamily", "iris")
        initial.setdefault("enabledPanels", [])
        background = initial.get("background", {})
        if not isinstance(background, dict):
            background = {}
        background["backend"] = {"provider": "ryogami"}
        initial["background"] = background
        try:
            with file.open("x") as out:
                json.dump(initial, out)
        except FileExistsError:
            pass
    return config, state, cache

def main():
    runtime = Path(os.environ["RYOKU_SHIMA_SOURCE"])
    args = sys.argv[1:]
    if args == ["--root"]:
        print(runtime)
        return
    if args == ["--version"]:
        print("Shima / iNiR " + (runtime / "VERSION").read_text().strip())
        return
    if os.environ.get("RYOKU_SHIMA_MANAGED") != "1" and not (args and args[0] == "ipc"):
        sys.exit("Shima must be supervised by ryoku-shell. Select Shima in Ryoku Hub.")
    if args and args[0] != "ipc":
        sys.exit("Expected no arguments or ipc")
    home = Path.home()
    config, state, cache = prepare(
        os.environ.get("XDG_CONFIG_HOME", str(home / ".config")),
        os.environ.get("XDG_STATE_HOME", str(home / ".local/state")),
        os.environ.get("XDG_CACHE_HOME", str(home / ".cache")))
    os.environ["RYOKU_SHIMA_CONFIG"] = str(config)
    os.environ["XDG_STATE_HOME"] = str(state)
    os.environ["XDG_CACHE_HOME"] = str(cache)
    os.environ["INIR_SYSTEM_RUNTIME_DIR"] = str(runtime)
    os.environ["INIR_RUNTIME_DIR"] = str(runtime)
    os.execvp("qs", ["qs", "-p", str(runtime)] + args)

if __name__ == "__main__":
    main()
