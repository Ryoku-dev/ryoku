#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Offline native packaging for the contained Hyakki source tree."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import unicodedata

ROOT = Path(__file__).resolve().parent.parent
SOURCE_ROOT = ROOT.parent.parent
MODULE = "ryoku/hyakki"
TREES = tuple(f"{MODULE}/{name}" for name in
              ("cmd", "internal", "vendor", "release", "systemd", "runit", "usage")) + (
                  "docs/lithic/library/go", "ryoku/i18n", "ryoku/wm/oniwindow",
                  "ryoku/wm/reflex")
ROOT_FILES = tuple(f"{MODULE}/{name}" for name in (
    ".gitignore", "go.mod", "go.sum", "LICENSE", "NOTICE", "NOTICE.upstream",
    "LICENSE.apache-2.0", "dev.ryoku.Hyakki.desktop",
    "dev.ryoku.Hyakki.metainfo.xml", "dev.ryoku.Hyakki.svg")) + ("LICENSE", "NOTICE")
LOCAL_MODULES = {
    "github.com/ryoku-dev/ryoku/docs/lithic/library/go":
        SOURCE_ROOT / "docs/lithic/library/go",
    "ryoku-i18n": SOURCE_ROOT / "ryoku/i18n",
    "ryoku-rashin/internal/oni/window": SOURCE_ROOT / "ryoku/wm/oniwindow",
    "ryoku-rashin/internal/reflex/wm": SOURCE_ROOT / "ryoku/wm/reflex",
}
NOTICES = ROOT / "release/notices/THIRD_PARTY_NOTICES"
SOURCE_MANIFEST = ROOT / "release/SOURCES"
LITHIC = ROOT / "vendor/github.com/ryoku-dev/ryoku/docs/lithic/library/go"
BINARIES = ("hyakki", "oni", "hyakki-app", "oni-surface")
COLLECTORS = ("hyakki-usage", "claude-usage", "codex-usage", "opencode-usage")
UNITS = ("oni.service", "oni.socket", "oni-routines.service", "oni-routines.timer",
         "hyakki-usage.service", "hyakki-usage.timer")
ONI_ASSETS = "release/oni-assets.json"
ONI_CHARACTERS = "internal/oni/characters"
ONI_TEXT = ("internal/oni", "internal/dictate", "internal/reflex", "internal/hands",
            "internal/mcpserve", "release/docs/oni-animation-spec.md")
ONI_SPDX = {"GPL-3.0-only", "GPL-3.0-or-later", "Apache-2.0", "MIT", "CC-BY-4.0",
            "CC0-1.0", "BSD-2-Clause", "BSD-3-Clause", "OFL-1.1"}
ONI_ENTRY_KEYS = {"path", "spdx", "license_file", "provenance", "source", "sha256"}
MODEL_SUFFIXES = (".onnx", ".bin", ".gguf", ".wav", ".ogg", ".opus", ".flac", ".mp3")
MODEL_DIRS = ("espeak-ng-data",)
# Reference products whose names, characters and artwork are never shipped.
FORBIDDEN_MARKS = (
    ("N\u016bs", re.compile("n\u016bs", re.IGNORECASE)),
    ("Knot", re.compile(r"\bKnots?\b")),
    ("Nixi", re.compile("nixi", re.IGNORECASE)),
    ("Coucou", re.compile("coucou", re.IGNORECASE)),
)


def source_name(name):
    path = Path(name)
    if (path.is_absolute() or path.as_posix() != name or "\\" in name
            or any(part in ("", ".", "..") for part in name.split("/"))
            or any(ord(char) < 32 for char in name)):
        raise ValueError(f"invalid source manifest path: {name!r}")
    if name not in ROOT_FILES and not any(name.startswith(tree + "/") for tree in TREES):
        raise ValueError(f"path is outside the distributed source: {name}")
    return name


def manifest():
    result = subprocess.run(
        ["git", "-C", str(SOURCE_ROOT), "ls-files", "-z", "--", *ROOT_FILES, *TREES],
        check=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode:
        raise ValueError("source manifest requires the reviewed Git index")
    names = {source_name(item) for item in result.stdout.decode("utf-8").split("\0") if item}
    names.discard(NOTICES.relative_to(SOURCE_ROOT).as_posix())
    names.add(SOURCE_MANIFEST.relative_to(SOURCE_ROOT).as_posix())
    if not set(ROOT_FILES).issubset(names):
        raise ValueError("stage the reviewed distribution files before generating release/SOURCES")
    if SOURCE_MANIFEST.is_symlink():
        raise ValueError("source manifest must not be a symlink")
    SOURCE_MANIFEST.write_text("".join(name + "\n" for name in sorted(names)), encoding="utf-8")
    print(f"Recorded {len(names)} reviewed source paths from the Git index")


def source_files():
    if not SOURCE_MANIFEST.is_file():
        raise ValueError("release/SOURCES is missing; stage reviewed files and run release/native.py manifest")
    listed = SOURCE_MANIFEST.read_text(encoding="utf-8").splitlines()
    approved = {source_name(name) for name in listed}
    if listed != sorted(approved):
        raise ValueError("release/SOURCES must be sorted without duplicate paths")
    required = {*ROOT_FILES, SOURCE_MANIFEST.relative_to(SOURCE_ROOT).as_posix()}
    if not required.issubset(approved):
        raise ValueError("release/SOURCES is missing required distribution inputs")
    actual = set()
    for name in ROOT_FILES:
        path = SOURCE_ROOT / name
        if path.is_symlink() or not path.is_file():
            raise ValueError(f"required source file is not regular: {path}")
        actual.add(name)
    for name in TREES:
        directory = SOURCE_ROOT / name
        if directory.is_symlink() or not directory.is_dir():
            raise ValueError(f"required source directory is not regular: {directory}")
        for parent, dirs, files in os.walk(directory, followlinks=False):
            dirs[:] = sorted(d for d in dirs if d not in (".git", "__pycache__"))
            for entry in sorted(dirs + files):
                path = Path(parent) / entry
                if path.is_symlink():
                    raise ValueError(f"source distribution cannot contain symlinks: {path}")
            for entry in sorted(files):
                path = Path(parent) / entry
                if entry.endswith((".pyc", ".pyo")) or path == NOTICES:
                    continue
                if not path.is_file():
                    raise ValueError(f"source distribution requires regular files: {path}")
                actual.add(path.relative_to(SOURCE_ROOT).as_posix())
    if actual != approved:
        raise ValueError(f"source manifest mismatch, missing={sorted(approved - actual)}, "
                         f"unexpected={sorted(actual - approved)}")
    for name in listed:
        yield SOURCE_ROOT / name


def replacements():
    block = False
    for original in (ROOT / "go.mod").read_text().splitlines():
        line = original.split("//", 1)[0].strip()
        if line == "replace (":
            block = True
            continue
        if block and line == ")":
            block = False
            continue
        if line.startswith("replace "):
            line = line[len("replace "):]
        elif not block:
            continue
        if "=>" not in line:
            continue
        before, after = line.split("=>", 1)
        fields = after.strip().split()
        if len(fields) == 1:
            relative = fields[0].strip('"')
            module = before.strip().split()[0]
            path = (ROOT / relative).resolve()
            if module not in LOCAL_MODULES or path != LOCAL_MODULES[module] or not path.is_dir():
                raise ValueError(f"local replacement must use its canonical shared source: {relative}")
            yield module, path


def check_source():
    list(source_files())
    check_oni_assets()
    local = dict(replacements())
    if set(local) != set(LOCAL_MODULES):
        raise ValueError("go.mod must use every canonical local module replacement")
    vendor_manifest = (ROOT / "vendor/modules.txt").read_text()
    for line in vendor_manifest.splitlines():
        if "=>" not in line:
            continue
        before, after = line.split("=>", 1)
        target = after.strip().split()
        if len(target) == 1:
            module = before.removeprefix("# ").strip().split()[0]
            path = (ROOT / target[0]).resolve()
            if module not in local or path != local[module]:
                raise ValueError("regenerate vendor after restoring canonical shared replacements")
    for module, origin in local.items():
        vendor = ROOT / "vendor" / module
        for path in vendor.rglob("*"):
            if not path.is_file():
                continue
            source = origin / path.relative_to(vendor)
            if not source.is_file() or source.read_bytes() != path.read_bytes():
                raise ValueError(f"vendored shared source drift: {path.relative_to(ROOT)}")


def oni_declarations(root):
    data = json.loads((root / ONI_ASSETS).read_text(encoding="utf-8"))
    if not isinstance(data, dict) or set(data) != {"version", "assets"} or data["version"] != 1:
        raise ValueError(f"{ONI_ASSETS} must be an object with version 1 and assets only")
    entries = data["assets"]
    if not isinstance(entries, list):
        raise ValueError(f"{ONI_ASSETS} assets must be a list")
    for entry in entries:
        if not isinstance(entry, dict) or set(entry) != ONI_ENTRY_KEYS:
            raise ValueError(f"{ONI_ASSETS} entry needs exactly {sorted(ONI_ENTRY_KEYS)}: {entry!r}")
    return data


def declared_path(root, name, field):
    if not isinstance(name, str) or not name:
        raise ValueError(f"{ONI_ASSETS} {field} must be a path: {name!r}")
    path = Path(name)
    if (path.is_absolute() or path.as_posix() != name
            or any(part in ("", ".", "..") for part in name.split("/"))
            or path.parts[0] not in ("internal", "vendor", "share")):
        raise ValueError(f"{ONI_ASSETS} {field} must be a relative module asset path: {name!r}")
    return root / path


def file_digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def is_model_or_voice(path):
    return (path.name.lower().endswith(MODEL_SUFFIXES)
            or any(part in MODEL_DIRS for part in path.parts))


def check_oni_assets(root=ROOT):
    """Match the Oni asset declarations against the files actually in the tree."""
    root = Path(root)
    problems = []
    entries = oni_declarations(root)["assets"]
    names = [entry["path"] for entry in entries]
    if names != sorted(set(names)):
        problems.append("declared asset paths must be sorted and unique")
    declared = set()
    for entry in entries:
        path = declared_path(root, entry["path"], "path")
        declared.add(entry["path"])
        if not isinstance(entry["spdx"], str) or entry["spdx"] not in ONI_SPDX:
            problems.append(f"undeclarable license {entry['spdx']!r} for {entry['path']}")
        if not isinstance(entry["source"], str) or not entry["source"].strip():
            problems.append(f"missing source for {entry['path']}")
        if path.is_symlink() or not path.is_file():
            problems.append(f"declared asset is missing: {entry['path']}")
        else:
            digest = entry["sha256"]
            if not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest):
                problems.append(f"digest not recorded for {entry['path']}, "
                                "run release/native.py oni-assets")
            elif digest != file_digest(path):
                problems.append(f"sha256 mismatch for {entry['path']}")
        license_file = declared_path(root, entry["license_file"], "license_file")
        if license_file.is_symlink() or not license_file.is_file():
            problems.append(f"declared license file is missing: {entry['license_file']}")
        elif not is_notice(license_file):
            problems.append(f"license file is not collected into THIRD_PARTY_NOTICES: "
                            f"{entry['license_file']}")
        elif entry["license_file"] not in names:
            problems.append(f"license file is not itself declared: {entry['license_file']}")
        provenance = declared_path(root, entry["provenance"], "provenance")
        if provenance.is_symlink() or not provenance.is_file():
            problems.append(f"declared provenance file is missing: {entry['provenance']}")
        elif entry["provenance"] not in names:
            problems.append(f"provenance file is not itself declared: {entry['provenance']}")
    characters = root / ONI_CHARACTERS
    for parent, dirs, files in os.walk(root, followlinks=False):
        if Path(parent) == root:
            dirs[:] = sorted(d for d in dirs if d in ("internal", "vendor", "share"))
        for entry in sorted(dirs + files):
            path = Path(parent) / entry
            name = path.relative_to(root).as_posix()
            inside = path.is_relative_to(characters)
            if inside and path.is_symlink():
                problems.append(f"symlink in shipped character assets: {name}")
                continue
            if entry not in files or name in declared:
                continue
            if inside and not entry.endswith(".go"):
                problems.append(f"undeclared character asset: {name}")
            elif is_model_or_voice(path.relative_to(root)):
                problems.append(f"undeclared model or voice data: {name}")
    scanned = []
    for name in ONI_TEXT:
        path = root / name
        if path.is_dir():
            scanned.extend(sorted(p for p in path.rglob("*") if p.is_file() and not p.is_symlink()))
        elif path.is_file():
            scanned.append(path)
    for path in scanned:
        text = unicodedata.normalize("NFKC", path.read_bytes().decode("utf-8", "replace"))
        for mark, pattern in FORBIDDEN_MARKS:
            if pattern.search(text):
                problems.append(f"forbidden mark {mark} in {path.relative_to(root).as_posix()}")
    notice = (root / "NOTICE").read_text(encoding="utf-8")
    if not any("Pranav Desu" in paragraph and "Apache-2.0" in paragraph
               and "N\u016bs Companion" in paragraph for paragraph in notice.split("\n\n")):
        problems.append("NOTICE lacks the Apache-2.0 attribution to Pranav Desu's "
                        "N\u016bs Companion")
    if problems:
        raise ValueError("Oni asset declarations: " + "; ".join(problems))


def record_oni_assets(root=ROOT):
    """Record the digests of already declared files; never declares new files."""
    root = Path(root)
    data = oni_declarations(root)
    for entry in data["assets"]:
        path = declared_path(root, entry["path"], "path")
        if path.is_symlink() or not path.is_file():
            raise ValueError(f"declared asset is missing: {entry['path']}")
        entry["sha256"] = file_digest(path)
    (root / ONI_ASSETS).write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n",
                                   encoding="utf-8")
    check_oni_assets(root)


def is_notice(path):
    name = path.name.lower()
    return (name == "attrib"
            or name.startswith(("attribution", "license", "licence", "copying", "copyright", "notice"))
            or name.endswith(("-ofl.txt", ".license", ".licence")))


def legal_files():
    paths = {ROOT / "NOTICE.upstream", ROOT / "LICENSE.apache-2.0",
             ROOT / "release/notices/go.LICENSE", SOURCE_ROOT / "LICENSE", SOURCE_ROOT / "NOTICE"}
    for directory in (ROOT / "vendor", ROOT / "internal", *LOCAL_MODULES.values()):
        for path in directory.rglob("*"):
            if path.is_file() and is_notice(path):
                paths.add(path)
    local = dict(replacements())
    for line in (ROOT / "vendor/modules.txt").read_text().splitlines():
        if not line.startswith("# ") or line.startswith("##"):
            continue
        module = line.split()[1]
        directory = ROOT / "vendor" / module
        if any(p.is_relative_to(directory) for p in paths):
            continue
        origin = local.get(module)
        if origin is not None and any(
                p.is_relative_to(origin) or p.parent in origin.parents for p in paths):
            continue
        raise ValueError(f"no retained license found for vendored module: {module}")
    for path in paths:
        if not path.is_file():
            raise ValueError(f"required license is missing: {path}")
    return sorted(paths, key=lambda p: p.relative_to(SOURCE_ROOT).as_posix())


def notice_bytes():
    sections = ["Hyakki bundled source and dependency notices\n\n"
                "Combined binaries linking Ryoku GPLv3-only code are distributed under\n"
                "GPL-3.0-only. Individual source grants below remain unchanged.\n"
                "This inventory includes all vendored module and embedded asset notices,\n"
                "including source not linked into every executable. See root NOTICE.\n"]
    for path in legal_files():
        data = path.read_bytes()
        heading = path.relative_to(SOURCE_ROOT).as_posix()
        digest = hashlib.sha256(data).hexdigest()
        sections.append(f"\n{'=' * 72}\n{heading}\nSHA256 {digest}\n{'=' * 72}\n\n"
                        + data.decode("utf-8").rstrip("\n") + "\n")
    return "".join(sections).encode("utf-8")


def notices(check=False):
    check_source()
    data = notice_bytes()
    if check:
        if not NOTICES.is_file() or NOTICES.read_bytes() != data:
            raise ValueError("third-party notices are missing or stale, run release/native.py notices")
    else:
        NOTICES.parent.mkdir(parents=True, exist_ok=True)
        NOTICES.write_bytes(data)


def install(destdir, build_dir):
    if not destdir.is_absolute():
        raise ValueError("DESTDIR must be absolute")
    destdir = destdir.resolve()
    if destdir == Path("/") or destdir in ROOT.parents or destdir == ROOT:
        raise ValueError("DESTDIR must be a staging directory, not a live or source root")
    if destdir.exists() and any(destdir.iterdir()):
        raise ValueError("DESTDIR must be empty to avoid mixing old packages or live paths")
    if not build_dir.is_absolute():
        raise ValueError("HYAKKI_BUILD_DIR must be absolute")
    notices(check=True)
    payload = []

    def add(source, target, mode=0o644):
        if source.is_symlink() or not source.is_file():
            raise ValueError(f"required package input is not a physical regular file: {source}")
        payload.append((source, target, mode))

    for binary in BINARIES:
        if not (build_dir / binary).stat().st_mode & 0o111:
            raise ValueError(f"required program is not executable: {build_dir / binary}")
        add(build_dir / binary, f"usr/bin/{binary}", 0o755)
    for collector in COLLECTORS:
        add(ROOT / "usage" / collector, f"usr/bin/{collector}", 0o755)
    for unit in UNITS:
        add(ROOT / "systemd" / unit, f"usr/lib/systemd/user/{unit}")
    for service in ("oni", "oni-routines", "hyakki-usage"):
        directory = ROOT / "runit" / service
        for path in sorted(directory.iterdir()):
            add(path, f"usr/share/hyakki/runit/{service}/{path.name}",
                0o755 if path.name in ("run", "finish") else 0o644)
    for suffix, subdir in (("desktop", "applications"), ("metainfo.xml", "metainfo")):
        name = f"dev.ryoku.Hyakki.{suffix}"
        add(ROOT / name, f"usr/share/{subdir}/{name}")
    add(ROOT / "dev.ryoku.Hyakki.svg",
        "usr/share/icons/hicolor/scalable/apps/dev.ryoku.Hyakki.svg")
    licenses = {
        "LICENSE": ROOT / "LICENSE", "NOTICE": ROOT / "NOTICE",
        "NOTICE.upstream": ROOT / "NOTICE.upstream",
        "LICENSE.apache-2.0": ROOT / "LICENSE.apache-2.0",
        "THIRD_PARTY_NOTICES": NOTICES,
        "LICENSE.material-symbols": LITHIC / "symbols/LICENSE",
        "LICENSE.geologica": LITHIC / "fonts/Geologica-OFL.txt",
        "LICENSE.martian-mono": LITHIC / "fonts/MartianMono-OFL.txt",
    }
    for name, path in licenses.items():
        add(path, f"usr/share/licenses/hyakki/{name}")
    for name in ("README.md", "native-distribution.md", "licensing.md", "oni-licensing.md",
                 "oni-animation-spec.md"):
        add(ROOT / "release/docs" / name, f"usr/share/doc/hyakki/{name}")
    add(ROOT / "release/docs/CHANGELOG", "usr/share/doc/hyakki/CHANGELOG")
    add(ROOT / "release/optional.json", "usr/share/hyakki/optional.json")
    for source, target, mode in payload:
        output = destdir / target
        output.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, output)
        output.chmod(mode)
    print(f"Staged Hyakki at {destdir}, no services were enabled or started")


def source_archive(output, version):
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._+-]*", version):
        raise ValueError("invalid source version")
    output = output.resolve()
    if any(output.is_relative_to(SOURCE_ROOT / tree) for tree in TREES):
        raise ValueError("source archive output must be outside the distributed source trees")
    notices()
    files = sorted([*source_files(), NOTICES], key=lambda p: p.relative_to(SOURCE_ROOT).as_posix())
    epoch = int(os.environ.get("SOURCE_DATE_EPOCH", "0"))
    if epoch < 0:
        raise ValueError("SOURCE_DATE_EPOCH must not be negative")
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("xb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=epoch) as compressed:
            with tarfile.open(fileobj=compressed, mode="w|", format=tarfile.PAX_FORMAT) as archive:
                for path in files:
                    name = f"hyakki-{version}/{path.relative_to(SOURCE_ROOT).as_posix()}"
                    info = tarfile.TarInfo(name)
                    data = path.read_bytes()
                    info.size = len(data)
                    info.mode = 0o755 if path.stat().st_mode & 0o111 else 0o644
                    info.mtime = epoch
                    archive.addfile(info, io.BytesIO(data))
    print(output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("check-source")
    commands.add_parser("manifest")
    notice_parser = commands.add_parser("notices")
    notice_parser.add_argument("--check", action="store_true")
    oni_parser = commands.add_parser("oni-assets")
    oni_parser.add_argument("--check", action="store_true")
    install_parser = commands.add_parser("install")
    install_parser.add_argument("--destdir", type=Path, required=True)
    install_parser.add_argument("--build-dir", type=Path, required=True)
    source_parser = commands.add_parser("source")
    source_parser.add_argument("--output", type=Path, required=True)
    source_parser.add_argument("--version", required=True)
    args = parser.parse_args()
    try:
        if args.command == "check-source":
            check_source()
        elif args.command == "manifest":
            manifest()
        elif args.command == "notices":
            notices(args.check)
        elif args.command == "oni-assets":
            if args.check:
                check_oni_assets()
            else:
                record_oni_assets()
        elif args.command == "install":
            install(args.destdir, args.build_dir)
        elif args.command == "source":
            source_archive(args.output, args.version)
    except (OSError, ValueError) as error:
        parser.exit(1, f"hyakki distribution: {error}\n")


if __name__ == "__main__":
    main()
