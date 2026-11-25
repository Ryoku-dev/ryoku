#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Prepare one private native builder, or build a frozen archive without network."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shlex
import shutil
import subprocess
import sys
import tarfile
import time
import uuid


VOID_REVISION = "988619731769a568ed4a48f6c688c3be23a4f165"


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def run(argv, **kwargs):
    print("+ " + shlex.join(map(str, argv)), flush=True)
    return subprocess.run(list(map(str, argv)), check=True, timeout=1800, **kwargs)


def sha256(path):
    checksum = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            checksum.update(chunk)
    return checksum.hexdigest()


def extract_source(destination):
    destination.mkdir()
    with tarfile.open("/input/source.tar.gz", "r:gz") as archive:
        members = archive.getmembers()
        require(sum(m.size for m in members) <= 2 * 1024 ** 3, "source expands beyond 2 GiB")
        roots = set()
        for member in members:
            path = PurePosixPath(member.name)
            require(not path.is_absolute() and ".." not in path.parts and path.parts,
                    "unsafe source member")
            require(member.isfile() or member.isdir(), "source must contain only files and directories")
            roots.add(path.parts[0])
        require(len(roots) == 1, "source must have one root")
        for member in members:
            target = destination / member.name
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            else:
                target.parent.mkdir(parents=True, exist_ok=True)
                with archive.extractfile(member) as source, target.open("wb") as out:
                    shutil.copyfileobj(source, out)
                target.chmod(0o755 if member.mode & 0o111 else 0o644)
    return destination / roots.pop()


def dependencies(source, distro):
    recipes = source / "ryoku/hyakki/release/packages"
    if distro == "arch":
        text = (recipes / "arch/PKGBUILD").read_text()
        values = []
        for name in ("depends", "makedepends"):
            block = re.search(r"^" + name + r"=\((.*?)\)", text, re.M | re.S)
            require(block is not None, f"missing Arch {name}")
            values.extend(shlex.split(block.group(1)))
        return values
    if distro == "fedora":
        return re.findall(r"^(?:BuildRequires|Requires):\s+(.+)$",
                          (recipes / "fedora/hyakki.spec").read_text(), re.M)
    text = (recipes / "void/template").read_text()
    values = []
    for name in ("hostmakedepends", "makedepends", "depends"):
        match = re.search(r'^' + name + r'="([^"]*)"', text, re.M)
        require(match is not None, f"missing Void {name}")
        values.extend(shlex.split(match.group(1)))
    return values


def guest(distro, prepare):
    require(Path("/.dockerenv").exists(), "guest mode requires the disposable container")
    source = extract_source(Path("/work/source"))
    deps = dependencies(source, distro)
    require(all(re.fullmatch(r"[A-Za-z0-9+._<>= -]+", dep) for dep in deps),
            "unsupported dependency expression; review the frozen recipe")
    if distro == "void":
        os.environ.update({"XBPS_ALLOW_CHROOT_BREAKOUT": "yes", "XBPS_CHROOT_CMD": "uchroot",
                           "XBPS_MAKEJOBS": "2"})
    if prepare:
        if distro == "arch":
            run(["pacman", "-S", "--needed", "--noconfirm", *deps,
                 "base-devel", "strace", "wtype", "util-linux"])
            run(["useradd", "--create-home", "builder"])
            run(["pacman", "-Q"])
            run(["pacman", "-Scc", "--noconfirm"])
        elif distro == "fedora":
            run(["dnf", "install", "-y", "--setopt=install_weak_deps=False", *deps,
                 "rpm-build", "strace", "wtype", "findutils", "tar", "gzip", "diffutils"])
            run(["rpm", "-qa"])
            run(["dnf", "clean", "all"])
        else:
            run(["xbps-install", "-y", *deps, "base-devel", "strace", "wtype", "util-linux", "git", "bash"])
            Path("/opt/void-packages").mkdir(parents=True, exist_ok=True)
            os.chown("/opt/void-packages", 0, 0)
            run(["git", "init", "/opt/void-packages"])
            run(["git", "-C", "/opt/void-packages", "remote", "add", "origin",
                 "https://github.com/void-linux/void-packages.git"])
            run(["git", "-C", "/opt/void-packages", "fetch", "--depth=1", "origin", VOID_REVISION])
            run(["git", "-C", "/opt/void-packages", "checkout", "--detach", "FETCH_HEAD"])
            run(["./xbps-src", "-m", "/opt/void-packages/masterdir", "binary-bootstrap"],
                cwd="/opt/void-packages")
            run(["xbps-install", "-r", "/opt/void-packages/masterdir", "-Sy", *deps])
            run(["xbps-query", "-l"])
        shutil.rmtree(source.parent)
        print("PRIVATE-BUILDER-PREPARED", flush=True)
        return
    os.environ.update({"GOMAXPROCS": "2", "GOFLAGS": "-p=2", "HOME": "/work/home",
                       "GOCACHE": "/work/go-cache", "GOTMPDIR": "/work/go-tmp"})
    for name in ("home", "go-cache", "go-tmp", "artifacts"):
        Path("/work", name).mkdir()
    if distro == "arch":
        recipe = Path("/work/arch")
        shutil.copytree(source / "ryoku/hyakki/release/packages/arch", recipe)
        version = re.search(r"^pkgver=(\S+)$", (recipe / "PKGBUILD").read_text(), re.M).group(1)
        require(source.name == "hyakki-" + version, "archive and PKGBUILD versions disagree")
        shutil.copyfile("/input/source.tar.gz", recipe / (source.name + ".tar.gz"))
        run(["chown", "-R", "builder:builder", "/work"])
        run(["runuser", "-u", "builder", "--", "makepkg", "--cleanbuild", "--noconfirm"], cwd=recipe)
        artifacts = list(recipe.glob("hyakki-*.pkg.tar.*"))
    elif distro == "fedora":
        top = Path("/work/rpmbuild")
        for name in ("BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"):
            (top / name).mkdir(parents=True)
        spec = source / "ryoku/hyakki/release/packages/fedora/hyakki.spec"
        version = re.search(r"^Version:\s+(\S+)$", spec.read_text(), re.M).group(1)
        require(source.name == "hyakki-" + version, "archive and spec versions disagree")
        shutil.copyfile("/input/source.tar.gz", top / "SOURCES" / (source.name + ".tar.gz"))
        run(["rpmbuild", "-ba", "--define", "_topdir /work/rpmbuild", spec])
        artifacts = list((top / "RPMS").rglob("hyakki-*.rpm"))
    else:
        tree = Path("/work/void-packages")
        revision = subprocess.check_output(
            ["git", "-C", "/opt/void-packages", "rev-parse", "HEAD"], text=True).strip()
        require(revision == VOID_REVISION, "prepared Void tree does not match the pinned revision")
        run(["cp", "-a", "/opt/void-packages", tree])
        # Dependencies and repository metadata remain installed. Archive caches
        # are redundant in this offline copy and compete with the compiler RAM.
        for cache in (tree / "hostdir").glob("repocache*"):
            if cache.is_dir():
                shutil.rmtree(cache)
        cache = tree / "masterdir/var/cache/xbps"
        if cache.is_dir():
            shutil.rmtree(cache)
        recipe = tree / "srcpkgs/hyakki"
        shutil.copytree(source / "ryoku/hyakki/release/packages/void", recipe)
        version = re.search(r"^version=(\S+)$", (recipe / "template").read_text(), re.M).group(1)
        require(source.name == "hyakki-" + version, "archive and template versions disagree")
        (recipe / "files").mkdir()
        shutil.copyfile("/input/source.tar.gz", recipe / "files" / (source.name + ".tar.gz"))
        shutil.rmtree(source.parent)
        for name in ("GOCACHE", "GOTMPDIR"):
            os.environ.pop(name, None)
        run(["./xbps-src", "-n", "-m", tree / "masterdir", "pkg", "hyakki"], cwd=tree)
        artifacts = list((tree / "hostdir/binpkgs").glob("hyakki-*.xbps"))
    require(len(artifacts) == 1, "expected exactly one real native package")
    target = Path("/work/artifacts") / artifacts[0].name
    shutil.copyfile(artifacts[0], target)
    print(json.dumps({"artifact": str(target), "sha256": sha256(target),
                      "bytes": target.stat().st_size}), flush=True)
    print("PRIVATE-PACKAGE-BUILT", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--distro", required=True, choices=("arch", "fedora", "void"))
    parser.add_argument("--image", required=True, help="pulled immutable image id; no implicit pull")
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--work", required=True, type=Path, help="new disposable build directory, preferably /dev/shm")
    parser.add_argument("--output", required=True, type=Path, help="new private log directory")
    parser.add_argument("--prepare", action="store_true", help="explicitly allow network dependency preparation and image commit")
    parser.add_argument("--timeout", type=int, default=3600)
    parser.add_argument("--cpuset-cpus", default="", help="optional Docker CPU affinity")
    parser.add_argument("--prepare-network", default="bridge",
                        help="Docker network used only for explicit dependency preparation")
    parser.add_argument("--void-tree", type=Path,
                        help="private prepared void-packages directory; writable for preparation, read-only for builds")
    args = parser.parse_args()
    require(re.fullmatch(r"sha256:[0-9a-f]{64}", args.image), "use an immutable local image id")
    require(60 <= args.timeout <= 3600, "timeout must be 60..3600 seconds")
    require(not args.cpuset_cpus or re.fullmatch(r"[0-9,-]+", args.cpuset_cpus),
            "invalid CPU affinity")
    require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", args.prepare_network)
            and args.prepare_network not in ("host", "container"),
            "use a private preparation network")
    if args.void_tree is not None:
        require(args.distro == "void" and args.void_tree.is_dir()
                and not args.void_tree.is_symlink(), "Void tree must be a private real directory")
    require(args.source.is_file() and not args.source.is_symlink(), "source archive must be a regular file")
    require(not args.work.exists() and not args.output.exists(), "work and output must be new directories")
    inspect = subprocess.check_output(["docker", "image", "inspect", args.image], timeout=30)
    args.work.mkdir(parents=True, mode=0o700)
    args.output.mkdir(parents=True, mode=0o700)
    (args.output / "image.json").write_bytes(inspect)
    request = vars(args).copy()
    for name in ("source", "work", "output"):
        request[name] = str(request[name].resolve())
    request["void_tree"] = str(args.void_tree.resolve()) if args.void_tree else None
    request["source_sha256"] = sha256(args.source)
    request["work_free_bytes"] = shutil.disk_usage(args.work).free
    (args.output / "request.json").write_text(json.dumps(request, indent=2) + "\n")
    name = "hyakki-package-build-" + uuid.uuid4().hex[:12]
    command = ["docker", "run", "--pull=never", "--name", name,
               "--memory=3g", "--memory-swap=3g", "--cpus=2", "--pids-limit=512",
               "--network=" + (args.prepare_network if args.prepare else "none"),
               "--mount", f"type=bind,src={args.work.resolve()},dst=/work",
               "--mount", f"type=bind,src={args.source.resolve()},dst=/input/source.tar.gz,readonly",
               "--mount", f"type=bind,src={Path(__file__).resolve()},dst=/package-build.py,readonly"]
    command += ["--oom-score-adj=1000"]
    if args.cpuset_cpus:
        command += ["--cpuset-cpus", args.cpuset_cpus]
    if args.distro == "void":
        command += ["--cap-add=SYS_ADMIN", "--security-opt=seccomp=unconfined"]
        command += ["--tmpfs", "/var/cache/xbps:rw,nosuid,nodev,size=512m"]
        if args.void_tree:
            command += ["--mount", f"type=bind,src={args.void_tree.resolve()},dst=/opt/void-packages"
                        + ("" if args.prepare else ",readonly")]
    if args.prepare:
        bootstrap = {"arch": "pacman -Syu --noconfirm python",
                     "fedora": "dnf install -y --setopt=install_weak_deps=False python3",
                     "void": "xbps-install -Sy python3"}[args.distro]
        script = bootstrap + "; exec python3 /package-build.py --guest-prepare " + args.distro
        command += ["--entrypoint", "/bin/sh", args.image, "-ec", script]
    else:
        command += ["--entrypoint", "python3", args.image, "/package-build.py", "--guest-build", args.distro]
    started = time.monotonic()
    try:
        with (args.output / "build.log").open("wb") as log:
            result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT, timeout=args.timeout)
        require(result.returncode == 0, "private package action failed; see build.log")
        state = subprocess.check_output(["docker", "container", "inspect", "--size", name], timeout=30)
        (args.output / "container-size.json").write_bytes(state)
        result = {"seconds": time.monotonic() - started, "passed": True}
        if args.prepare:
            result["prepared_image"] = subprocess.check_output(["docker", "commit", name], timeout=120).decode().strip()
        else:
            result["artifacts"] = [{"path": str(p), "bytes": p.stat().st_size, "sha256": sha256(p)}
                                   for p in (args.work / "artifacts").iterdir()]
        (args.output / "result.json").write_text(json.dumps(result, indent=2) + "\n")
    finally:
        subprocess.run(["docker", "rm", "-f", name], stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL, timeout=30, check=False)


if __name__ == "__main__":
    try:
        if len(sys.argv) == 3 and sys.argv[1] in ("--guest-prepare", "--guest-build"):
            guest(sys.argv[2], sys.argv[1] == "--guest-prepare")
        else:
            main()
    except (OSError, RuntimeError, ValueError, KeyError, subprocess.SubprocessError, tarfile.TarError) as error:
        sys.exit(f"hyakki package build: {error}")
