#!/usr/bin/env python3
import argparse
import json
import os
import sys
from pathlib import Path

DEFAULT_INHERITS = "Adwaita"
THEME_PREFIX = "Bibata-Material-"


def display_name(theme_key: str) -> str:
    return "Material Bibata Cursor " + theme_key.replace("-", " ")


def validate_theme_dir(theme_dir: Path) -> list[str]:
    problems = []
    if not theme_dir.is_dir():
        problems.append(f"directory does not exist: {theme_dir}")
        return problems
    cursors_dir = theme_dir / "cursors"
    if not cursors_dir.is_dir():
        problems.append("missing 'cursors/' subdirectory")
    elif not any(cursors_dir.iterdir()):
        problems.append("'cursors/' subdirectory is empty")
    return problems


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--install-dir", default=os.environ.get(
        "BIBATA_MATERIAL_INSTALL_DIR", str(Path.home() / ".icons")))
    parser.add_argument("--themes-json", required=True)
    parser.add_argument("--inherits", default=DEFAULT_INHERITS)
    parser.add_argument("--theme", required=True)
    args = parser.parse_args()

    themes_path = Path(args.themes_json)
    themes = json.loads(themes_path.read_text(encoding="utf-8"))
    if args.theme not in themes:
        sys.exit(f"Error: '{args.theme}' not found in {themes_path}")

    theme_dir = Path(args.install_dir) / f"{THEME_PREFIX}{args.theme}"
    problems = validate_theme_dir(theme_dir)
    if problems:
        sys.exit("; ".join(problems))
    content = (
        "[Icon Theme]\n"
        f"Name={display_name(args.theme)}\n"
        "Comment=Material Bibata Cursor\n"
        f"Inherits={args.inherits}\n"
    )
    (theme_dir / "index.theme").write_text(content, encoding="utf-8", newline="\n")


if __name__ == "__main__":
    main()
