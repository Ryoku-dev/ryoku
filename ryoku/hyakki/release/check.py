#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Check distribution metadata without running Hyakki or touching user services."""
import ast
import json
from pathlib import Path
import re
import sys
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parent.parent
IDENTITY = "dev.ryoku.Hyakki"
UNITS = {"oni.socket", "oni.service", "oni-routines.service", "oni-routines.timer",
         "hyakki-usage.service", "hyakki-usage.timer"}
DISTROS = ("arch", "fedora", "void", "nix")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def json_tags():
    """Every json field name the wire protocol declares."""
    tags = set()
    for path in ROOT.joinpath("internal/protocol").glob("*.go"):
        if not path.name.endswith("_test.go"):
            tags.update(re.findall(r'json:"([A-Za-z]+)', path.read_text(encoding="utf-8")))
    return tags


def check_optional():
    """release/optional.json is the one list of optional programs; the docs and
    every recipe must name exactly what it says."""
    table = json.loads((ROOT / "release/optional.json").read_text(encoding="utf-8"))
    require(table.get("version") == 1, "release/optional.json: unknown version")
    keys = {"id", "kind", "programs", "features", "license", "why", "packages",
            "removable_by_hyakki", "default"}
    tags = json_tags()
    licensing = "".join((ROOT / "release/docs" / name).read_text(encoding="utf-8")
                        for name in ("oni-licensing.md", "licensing.md"))
    distribution = (ROOT / "release/docs/native-distribution.md").read_text(encoding="utf-8")
    seen = set()
    wanted = {distro: {} for distro in DISTROS}
    for c in table["components"]:
        require(set(c) == keys, f"optional component fields: {c.get('id')}")
        require(c["id"] not in seen, f"duplicate optional component: {c['id']}")
        seen.add(c["id"])
        require(c["kind"] == "program" and c["default"] == "off" and c["removable_by_hyakki"] is False,
                f"optional component must be an off-by-default program: {c['id']}")
        require(c["why"] and c["license"] and c["programs"] and c["features"],
                f"optional component lacks a reason, licence, program or feature: {c['id']}")
        require(set(c["packages"]) == set(DISTROS), f"optional component distros: {c['id']}")
        for feature in c["features"]:
            require(all(part in tags for part in feature.split(".")),
                    f"optional component {c['id']} names an unknown setting: {feature}")
        for program in c["programs"]:
            require(f"`{program}`" in licensing, f"the licensing docs have no row for `{program}`")
        require(f"\n| `{c['id']}` |" in distribution, f"docs/native-distribution.md has no row for `{c['id']}`")
        row = next(line for line in distribution.splitlines()
                   if line.startswith(f"| `{c['id']}` |"))
        cells = [cell.strip() for cell in row.split("|")[1:-1]]
        require(len(cells) == 7, f"optional table columns: {c['id']}")
        require(re.findall(r"`([^`]+)`", cells[1]) == c["programs"],
                f"optional table programs differ: {c['id']}")
        for index, distro in enumerate(DISTROS, 3):
            require(re.findall(r"`([^`]+)`", cells[index]) == c["packages"][distro],
                    f"optional table {distro} packages differ: {c['id']}")
        for distro in DISTROS:
            for package in c["packages"][distro]:
                wanted[distro][package] = c["why"]
    recipes = ROOT / "release/packages"
    pkgbuild = (recipes / "arch/PKGBUILD").read_text(encoding="utf-8")
    block = re.search(r"^optdepends=\((.*?)^\)", pkgbuild, re.S | re.M)
    require(block is not None, "PKGBUILD has no optdepends")
    arch = {}
    for line in block.group(1).splitlines():
        line = line.strip()
        if line:
            text = line.replace("'\"'\"'", "'").strip("'")
            name, why = text.split(": ", 1)
            arch[name] = why
    require(arch == wanted["arch"], "PKGBUILD optdepends differ from release/optional.json")
    required = re.search(r"^depends=\((.*?)^\)", pkgbuild, re.S | re.M)
    require(required is not None, "PKGBUILD has no required dependencies")
    require(not set(re.findall(r"'([^']+)'", required.group(1))) & set(arch),
            "an optional Arch program is also a required dependency")
    spec = (recipes / "fedora/hyakki.spec").read_text(encoding="utf-8")
    fedora = set(re.findall(r"^Suggests:\s+(\S+)$", spec, re.M))
    require(fedora == set(wanted["fedora"]), "hyakki.spec Suggests differ from release/optional.json")
    require(not set(re.findall(r"^Requires:\s+(\S+)", spec, re.M)) & fedora,
            "an optional Fedora program is also a required dependency")
    void = {}
    for line in (recipes / "void/INSTALL.msg").read_text(encoding="utf-8").splitlines():
        name, sep, why = line.partition(": ")
        if sep and " " not in name:
            void[name] = why
    require(void == wanted["void"], "void INSTALL.msg differs from release/optional.json")
    template = (recipes / "void/template").read_text(encoding="utf-8")
    required = re.search(r'^depends="([^"]*)"', template, re.M)
    require(required is not None, "Void template has no required dependencies")
    require(not set(required.group(1).split()) & set(void),
            "an optional Void program is also a required dependency")
    nix = (recipes / "nix/hyakki.nix").read_text(encoding="utf-8")
    require("../../optional.json" in nix, "hyakki.nix must read release/optional.json")


def main():
    for path in [*ROOT.joinpath("release").glob("*.py"),
                 *ROOT.joinpath("usage").glob("*-usage")]:
        if path.name != "hyakki-usage":
            ast.parse(path.read_text(), filename=str(path))
    component = ET.parse(ROOT / f"{IDENTITY}.metainfo.xml").getroot()
    require(component.findtext("id") == IDENTITY, "wrong AppStream identity")
    require(component.findtext("project_license") == "GPL-3.0-only", "wrong combined license")
    launchable = component.find("launchable")
    require(launchable is not None and launchable.text == f"{IDENTITY}.desktop",
            "wrong AppStream desktop launcher")
    desktop = (ROOT / f"{IDENTITY}.desktop").read_text()
    for line in ("Exec=hyakki-app", "TryExec=hyakki-app", f"Icon={IDENTITY}",
                 f"StartupWMClass={IDENTITY}"):
        require(line in desktop.splitlines(), f"desktop file is missing {line}")
    ET.parse(ROOT / f"{IDENTITY}.svg")
    units = ROOT / "systemd"
    require({p.name for p in units.iterdir()} == UNITS, "unexpected user unit names")
    require("ListenStream=%t/hyakki/oni.sock" in (units / "oni.socket").read_text(),
            "wrong daemon socket path")
    daemon = (units / "oni.service").read_text()
    require("ExecStart=/usr/bin/oni" in daemon and "Restart=no" in daemon,
            "daemon must remain on-demand and idle-exitable")
    for path in units.iterdir():
        require("ryoku-host" not in path.read_text(), "unit requires Ryoku host helper")
    require(not (ROOT / "usage/ryoku-ai-usage").exists(), "old usage entrypoint remains")
    inventory = (ROOT / "release/notices/THIRD_PARTY_NOTICES").read_text(encoding="utf-8")
    declared = json.loads((ROOT / "release/oni-assets.json").read_text(encoding="utf-8"))
    for name in sorted({entry["license_file"] for entry in declared["assets"]}):
        require(f"\nryoku/hyakki/{name}\nSHA256 " in inventory,
                f"declared Oni license is not in THIRD_PARTY_NOTICES: {name}")
    check_optional()
    if len(sys.argv) == 2:
        stage = Path(sys.argv[1]).resolve()
        expected = {
            "usr/bin/hyakki", "usr/bin/oni", "usr/bin/hyakki-app", "usr/bin/oni-surface",
            "usr/share/hyakki/optional.json",
            "usr/bin/hyakki-usage", "usr/bin/claude-usage", "usr/bin/codex-usage",
            "usr/bin/opencode-usage", f"usr/share/applications/{IDENTITY}.desktop",
            f"usr/share/metainfo/{IDENTITY}.metainfo.xml",
            f"usr/share/icons/hicolor/scalable/apps/{IDENTITY}.svg",
            "usr/share/doc/hyakki/README.md", "usr/share/doc/hyakki/oni-animation-spec.md",
            "usr/share/doc/hyakki/native-distribution.md", "usr/share/doc/hyakki/licensing.md",
            "usr/share/doc/hyakki/oni-licensing.md", "usr/share/doc/hyakki/CHANGELOG",
        }
        expected.update(f"usr/lib/systemd/user/{unit}" for unit in UNITS)
        expected.update(f"usr/share/licenses/hyakki/{name}" for name in (
            "LICENSE", "NOTICE", "NOTICE.upstream", "LICENSE.apache-2.0", "THIRD_PARTY_NOTICES",
            "LICENSE.material-symbols", "LICENSE.geologica", "LICENSE.martian-mono"))
        for service in ("oni", "oni-routines", "hyakki-usage"):
            for path in (ROOT / "runit" / service).iterdir():
                expected.add(f"usr/share/hyakki/runit/{service}/{path.name}")
            require((stage / f"usr/share/hyakki/runit/{service}/down").is_file(),
                    f"{service} is not dormant")
        actual = {p.relative_to(stage).as_posix() for p in stage.rglob("*") if p.is_file()}
        require(actual == expected,
                f"stage file mismatch, missing={sorted(expected - actual)}, extra={sorted(actual - expected)}")
        require(not any(p.is_symlink() for p in stage.rglob("*")),
                "stage unexpectedly links services or external files")
        for path in stage.joinpath("usr/bin").iterdir():
            require(path.stat().st_mode & 0o111, f"not executable: {path}")
        for name, source in (("LICENSE", ROOT / "LICENSE"), ("NOTICE", ROOT / "NOTICE"),
                             ("NOTICE.upstream", ROOT / "NOTICE.upstream"),
                             ("LICENSE.apache-2.0", ROOT / "LICENSE.apache-2.0"),
                             ("THIRD_PARTY_NOTICES", ROOT / "release/notices/THIRD_PARTY_NOTICES")):
            require((stage / "usr/share/licenses/hyakki" / name).read_bytes() == source.read_bytes(),
                    f"packaged license differs: {name}")
        require((stage / "usr/share/doc/hyakki/oni-licensing.md").read_bytes()
                == (ROOT / "release/docs/oni-licensing.md").read_bytes(), "packaged Oni licensing differs")
    print("Hyakki distribution checks passed, no services were started")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError, SyntaxError, ET.ParseError) as error:
        sys.exit(f"hyakki distribution check: {error}")
