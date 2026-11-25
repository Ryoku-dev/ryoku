#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Exercise real private package artifacts in an already prepared disposable image."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import time
import uuid


UNITS = ("oni.socket", "oni.service", "oni-routines.service", "oni-routines.timer",
         "hyakki-usage.service", "hyakki-usage.timer")
PROGRAMS = ("hyakki", "oni", "hyakki-app", "oni-surface")


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def run(argv, **kwargs):
    print("+ " + " ".join(map(str, argv)), flush=True)
    return subprocess.run(list(map(str, argv)), check=True, timeout=120, **kwargs)


def package_run(argv):
    trace = Path("/tmp") / ("package-exec-" + uuid.uuid4().hex + ".log")
    try:
        result = run(["strace", "-f", "-qq", "-e", "trace=execve", "-s", "512",
                      "-o", trace, *argv])
    finally:
        if trace.exists():
            text = trace.read_text()
            print(text, end="", flush=True)
            trace.unlink()
            for line in text.splitlines():
                match = re.search(r'execve\("([^"]+)", \[(.*?)\]', line)
                if match is None:
                    continue
                name = Path(match.group(1)).name
                args = set(re.findall(r'"([^"]*)"', match.group(2)))
                require(name not in (*PROGRAMS, ".oni-wrapped", "runsv", "runsvdir"),
                        f"package transaction launched a service: {line}")
                require(not (name == "systemctl" and args & {
                    "start", "restart", "enable", "reenable", "preset", "preset-all",
                    "try-restart", "reload-or-restart", "reload-or-try-restart"}),
                    f"package transaction activated a unit: {line}")
                require(not (name == "sv" and args & {"up", "once", "start", "restart"}),
                        f"package transaction activated a service: {line}")
    return result


def digest(path):
    checksum = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            checksum.update(chunk)
    return checksum.hexdigest()


def source_contract(path, distro, optional):
    lithic = "ryoku/hyakki/vendor/github.com/ryoku-dev/ryoku/docs/lithic/library/go"
    files = {"LICENSE": "ryoku/hyakki/LICENSE", "NOTICE": "ryoku/hyakki/NOTICE", "NOTICE.upstream": "ryoku/hyakki/NOTICE.upstream",
             "LICENSE.apache-2.0": "ryoku/hyakki/LICENSE.apache-2.0",
             "THIRD_PARTY_NOTICES": "ryoku/hyakki/release/notices/THIRD_PARTY_NOTICES",
             "LICENSE.material-symbols": lithic + "/symbols/LICENSE",
             "LICENSE.geologica": lithic + "/fonts/Geologica-OFL.txt",
             "LICENSE.martian-mono": lithic + "/fonts/MartianMono-OFL.txt"}
    with tarfile.open(path, "r:gz") as archive:
        names = archive.getnames()
        prefixes = {name.split("/")[0] for name in names}
        require(len(prefixes) == 1, "source archive must have one root")
        prefix = prefixes.pop() + "/"
        def read_member(name):
            member = archive.getmember(prefix + name)
            require(member.isfile() and member.size <= 16 * 1024 * 1024,
                    f"source contract input must be a regular file <=16 MiB: {name}")
            with archive.extractfile(member) as stream:
                return stream.read()
        table = json.loads(read_member("ryoku/hyakki/release/optional.json"))
        if distro != "nix":
            require(optional in {p for c in table["components"] for p in c["packages"][distro]},
                    "optional package must be declared by the frozen source")
        return {name: hashlib.sha256(read_member(source)).hexdigest()
                for name, source in files.items()}


def service_state():
    roots = ("/etc/systemd/user", "/etc/systemd/system", "/etc/service", "/var/service",
             "/etc/sv", "/root/.config/systemd/user", "/root/.config/service")
    result = {}
    for root in roots:
        base = Path(root)
        if base.exists():
            for path in base.rglob("*"):
                if path.is_symlink():
                    result[str(path)] = os.readlink(path)
                elif path.is_file():
                    result[str(path)] = digest(path)
    return result


def no_children():
    for path in Path("/proc").glob("[0-9]*/exe"):
        try:
            executable = Path(os.readlink(path)).name
        except (FileNotFoundError, PermissionError, ProcessLookupError):
            continue
        require(executable not in (*PROGRAMS, ".oni-wrapped"),
                f"install/remove left an application running: {path} {executable}")
    require(not list(Path("/tmp").rglob("oni.sock")), "install/remove created an Oni socket")


def guest():
    require(Path("/.dockerenv").exists() or Path("/run/.containerenv").exists(),
            "guest mode must run inside the disposable container")
    config = json.loads(Path("/proof/request.json").read_text())
    distro = config["distro"]
    sentinels = {}
    for root in ("/root/.config/hyakki", "/root/.local/share/hyakki",
                 "/root/.local/state/hyakki", "/root/.cache/hyakki"):
        path = Path(root) / "package-proof-keep"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("private user data, not package owned\n")
        sentinels[str(path)] = digest(path)
    settings = Path("/root/.config/hyakki/hyakki.json")
    settings.write_text("{}\n")
    sentinels[str(settings)] = digest(settings)
    link = Path("/root/.config/systemd/user/default.target.wants/oni.socket")
    link.parent.mkdir(parents=True, exist_ok=True)
    link.symlink_to("/usr/lib/systemd/user/oni.socket")
    baseline = service_state()
    no_children()
    prefix = Path("/usr")
    if distro == "nix":
        for archive in ("/inputs/old", "/inputs/new"):
            with open(archive, "rb") as stream:
                run(["nix-store", "--import"], stdin=stream)
        profile = "/tmp/hyakki-profile"
        old, new = config["old_store"], config["new_store"]
        prefix = Path(profile)

        def install(artifact, rollback=False):
            package_run(["nix-env", "--profile", profile, "--set", artifact])

        def remove():
            package_run(["nix-env", "--profile", profile, "--uninstall", "hyakki"])

        def inventory():
            run(["nix-env", "--profile", profile, "--query", "--out-path"])
    else:
        old, new = "/inputs/old", "/inputs/new"
        if distro == "arch":
            run(["pacman", "-Q"])
            def install(artifact, rollback=False):
                package_run(["pacman", "-U", "--noconfirm", artifact])
            def remove():
                package_run(["pacman", "-R", "--noconfirm", "hyakki"])
            def inventory():
                run(["pacman", "-Qil", "hyakki"])
        elif distro == "fedora":
            run(["rpm", "-qa"])
            def install(artifact, rollback=False):
                package_run(["rpm", "-Uvh", *(["--oldpackage"] if rollback else []), artifact])
            def remove():
                package_run(["rpm", "-e", "hyakki"])
            def inventory():
                run(["rpm", "-qil", "hyakki"])
        else:
            repo = Path("/tmp/xbps-repository")
            repo.mkdir()
            def install(artifact, rollback=False):
                version = (config["old_package_version"] if artifact == old
                           else config["new_package_version"])
                name = "hyakki-" + version + ".x86_64.xbps"
                for previous in repo.glob("*"):
                    previous.unlink()
                shutil.copyfile(artifact, repo / name)
                run(["xbps-rindex", "-a", repo / name])
                package_run(["xbps-install", "-y", "-f", "--repository", repo, "hyakki-" + version])
            def remove():
                package_run(["xbps-remove", "-y", "hyakki"])
            def inventory():
                run(["xbps-query", "-S", "hyakki"])
                run(["xbps-query", "-f", "hyakki"])

    expected_grants = {}
    def state(label, installed, version=None):
        no_children()
        require(service_state() == baseline, f"{label}: install/remove changed user service links or files")
        for path, checksum in sentinels.items():
            require(Path(path).is_file() and digest(path) == checksum,
                    f"{label}: user data changed: {path}")
        if installed:
            inventory()
            for name in PROGRAMS:
                binary = prefix / "bin" / name
                require(not binary.is_symlink() and binary.is_file() and os.access(binary, os.X_OK),
                        f"missing physical executable: {binary}")
                with binary.open("rb") as stream:
                    require(stream.read(4) == b"\x7fELF", f"program is not a native ELF: {binary}")
                print(json.dumps({"step": label, "binary": str(binary),
                                  "sha256": digest(binary)}), flush=True)
                result = run([binary, "--version"], stdout=subprocess.PIPE, text=True)
                print(result.stdout, end="", flush=True)
                require(version in result.stdout.split(), f"wrong installed version: {name}")
            for name in ("hyakki-app", "oni-surface"):
                result = run(["ldd", prefix / "bin" / name], stdout=subprocess.PIPE, text=True)
                print(result.stdout, end="", flush=True)
                require("not found" not in result.stdout, f"{name}: unresolved runtime library")
            grants = prefix / "share/licenses/hyakki"
            for name in ("LICENSE", "NOTICE", "NOTICE.upstream", "LICENSE.apache-2.0",
                         "THIRD_PARTY_NOTICES", "LICENSE.material-symbols", "LICENSE.geologica",
                         "LICENSE.martian-mono"):
                require((grants / name).is_file() and (grants / name).stat().st_size > 0,
                        f"missing installed grant: {name}")
            current = {p.name: digest(p) for p in grants.iterdir() if p.is_file()}
            if label in ("upgrade", "optional-program-removed", "reinstall"):
                require(current == config["source_grants"],
                        f"{label}: installed grants differ from the frozen corresponding source")
            if label == "install-old":
                expected_grants.update(current)
            if label == "rollback":
                require(current == expected_grants, "rollback did not restore the old grants")
            for unit in UNITS:
                require((prefix / "lib/systemd/user" / unit).is_file(), f"missing dormant unit: {unit}")
            for service in ("oni", "oni-routines", "hyakki-usage"):
                require((prefix / "share/hyakki/runit" / service / "down").is_file(),
                        f"missing dormant runit marker: {service}")
        else:
            require(not any((prefix / "bin" / name).exists() for name in PROGRAMS),
                    f"{label}: removed package still exposes a binary")
        no_children()
        print(json.dumps({"step": label, "passed": True}), flush=True)

    state("baseline", False)
    install(old)
    state("install-old", True, config["old_version"])
    install(new)
    state("upgrade", True, config["new_version"])
    optional = config["optional_package"]
    if distro != "nix":
        commands = {"arch": ["pacman", "-R", "--noconfirm", optional],
                    "fedora": ["rpm", "-e", optional],
                    "void": ["xbps-remove", "-y", optional]}
        package_run(commands[distro])
        state("optional-program-removed", True, config["new_version"])
    if distro == "nix":
        package_run(["nix-env", "--profile", "/tmp/hyakki-profile", "--rollback"])
        require(str(prefix.resolve()) == old, "profile rollback selected the wrong closure")
    else:
        install(old, rollback=True)
    state("rollback", True, config["old_version"])
    remove()
    state("remove", False)
    install(new)
    state("reinstall", True, config["new_version"])
    remove()
    state("final-remove", False)
    print("PACKAGED-LIFECYCLE-PASS", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--distro", required=True, choices=("arch", "fedora", "void", "nix"))
    parser.add_argument("--image", required=True, help="already present immutable sha256 image id")
    parser.add_argument("--old", required=True, type=Path, help="old package or exported Nix closure")
    parser.add_argument("--new", required=True, type=Path, help="new package or exported Nix closure")
    parser.add_argument("--source", required=True, type=Path, help="new frozen corresponding-source tar.gz")
    parser.add_argument("--old-version", required=True, help="version printed by the old binaries")
    parser.add_argument("--new-version", required=True, help="version printed by the new binaries")
    parser.add_argument("--old-package-version", default="", help="Void version_revision")
    parser.add_argument("--new-package-version", default="", help="Void version_revision")
    parser.add_argument("--old-store", default="", help="old Nix output store path")
    parser.add_argument("--new-store", default="", help="new Nix output store path")
    parser.add_argument("--optional-package", default="wtype", help="preinstalled removable optional package")
    parser.add_argument("--output", required=True, type=Path, help="new private evidence directory")
    parser.add_argument("--timeout", type=int, default=900)
    parser.add_argument("--cpuset-cpus", default="", help="optional Docker CPU affinity")
    parser.add_argument("--minimum-free-mib", type=int, default=0,
                        help="operator-selected measured Docker storage reserve, no fixed image-size guess")
    args = parser.parse_args()
    require(re.fullmatch(r"sha256:[0-9a-f]{64}", args.image), "select an immutable local image id")
    require(60 <= args.timeout <= 1800, "timeout must be 60..1800 seconds")
    require(args.minimum_free_mib >= 0, "minimum free storage cannot be negative")
    require(not args.cpuset_cpus or re.fullmatch(r"[0-9,-]+", args.cpuset_cpus),
            "invalid CPU affinity")
    require(re.fullmatch(r"[a-z0-9][a-z0-9+._-]*", args.optional_package), "invalid optional package")
    require(args.old_version != args.new_version, "use two distinct real builds for upgrade and rollback")
    for path in (args.old, args.new, args.source):
        require(path.is_file() and not path.is_symlink() and path.stat().st_size <= 1024 ** 3,
                f"artifact must be a regular file no larger than 1 GiB: {path}")
    require(digest(args.old) != digest(args.new), "old and new artifacts must differ")
    if args.distro == "void":
        for version in (args.old_package_version, args.new_package_version):
            require(re.fullmatch(r"[0-9][a-zA-Z0-9.+_-]*_[0-9]+", version), "supply both Void version_revision values")
    if args.distro == "nix":
        for path in (args.old_store, args.new_store):
            require(re.fullmatch(r"/nix/store/[a-z0-9]{32}-hyakki-[a-zA-Z0-9.+_-]+", path), "supply exact Hyakki store paths")
        require(args.old_store != args.new_store, "Nix outputs must differ")
    engine = shutil.which("docker")
    require(engine is not None, "docker is required; no image will be pulled")
    inspect = subprocess.check_output([engine, "image", "inspect", args.image], timeout=30)
    info = json.loads(inspect)[0]
    docker_info = json.loads(subprocess.check_output([engine, "info", "--format", "{{json .}}"], timeout=30))
    storage = Path(docker_info["DockerRootDir"])
    require(storage.exists(), "Docker storage is not locally measurable; run on the local engine host")
    free_bytes = shutil.disk_usage(storage).free
    require(free_bytes >= args.minimum_free_mib * 1024 ** 2,
            "Docker storage is below the requested measured reserve")
    require(not args.output.exists(), "evidence output already exists")
    args.output.mkdir(parents=True, mode=0o700)
    request = vars(args).copy()
    for key in ("old", "new", "source", "output"):
        request[key] = str(request[key].resolve())
    request["artifact_sha256"] = {"old": digest(args.old), "new": digest(args.new)}
    request["source_sha256"] = digest(args.source)
    request["source_grants"] = source_contract(args.source, args.distro, args.optional_package)
    request["image_id"] = info["Id"]
    request["docker_storage_free_bytes"] = free_bytes
    request["prepared_image_bytes"] = info["Size"]
    request["artifact_bytes"] = {"old": args.old.stat().st_size, "new": args.new.stat().st_size}
    (args.output / "request.json").write_text(json.dumps(request, indent=2) + "\n")
    (args.output / "image.json").write_bytes(inspect)
    name = "hyakki-package-proof-" + uuid.uuid4().hex[:12]
    command = [engine, "run", "--rm", "--pull=never", "--name", name, "--network=none",
               "--memory=3g", "--memory-swap=3g", "--cpus=2", "--pids-limit=512",
               "--oom-score-adj=1000",
               "--security-opt=no-new-privileges", "--cap-add=SYS_PTRACE",
               "--ulimit", "fsize=1073741824:1073741824",
               "--tmpfs", "/tmp:rw,nosuid,nodev,size=1g", "--env", "HOME=/root",
               "--env", "XDG_RUNTIME_DIR=/tmp/hyakki-runtime", "--env", "NIX_REMOTE=local",
               "--env", "NIX_CONFIG=substituters =\nbuilders =",
               "--mount", f"type=bind,src={Path(__file__).resolve()},dst=/proof-driver.py,readonly",
               "--mount", f"type=bind,src={args.output.resolve()},dst=/proof,readonly",
               "--mount", f"type=bind,src={args.old.resolve()},dst=/inputs/old,readonly",
               "--mount", f"type=bind,src={args.new.resolve()},dst=/inputs/new,readonly",
               "--entrypoint", "python3", args.image, "/proof-driver.py", "--guest"]
    if args.cpuset_cpus:
        command[2:2] = ["--cpuset-cpus", args.cpuset_cpus]
    start = time.monotonic()
    try:
        with (args.output / "lifecycle.log").open("wb") as log:
            result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT, timeout=args.timeout)
        require(result.returncode == 0, f"package proof failed ({result.returncode}); see lifecycle.log")
        require("PACKAGED-LIFECYCLE-PASS" in (args.output / "lifecycle.log").read_text(), "incomplete package proof")
        (args.output / "result.json").write_text(json.dumps({"passed": True, "seconds": time.monotonic() - start}) + "\n")
    finally:
        subprocess.run([engine, "rm", "-f", name], stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL, timeout=30, check=False)


if __name__ == "__main__":
    try:
        if sys.argv[1:] == ["--guest"]:
            guest()
        else:
            main()
    except (OSError, RuntimeError, subprocess.SubprocessError, ValueError, KeyError, tarfile.TarError) as error:
        sys.exit(f"hyakki package proof: {error}")
