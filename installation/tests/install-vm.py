#!/usr/bin/env python3
# Boot a Ryoku ISO in QEMU, run the installer unattended against a virtual disk,
# and verify the result, so a broken install or a missing package is caught
# before a user hits it. Drives the live root shell over the serial console
# (pexpect); the backend is env-driven and prints @@RYOKU_DONE on success.
#
#   install-vm.py --iso ryoku.iso            full install + verify
#   install-vm.py --iso ryoku.iso --boot-only   just reach the live shell (quick check)
#
# Exits 0 when the install completes and the installed tree checks out, non-zero
# otherwise, printing the serial log tail. Uses KVM when /dev/kvm is present,
# else TCG (slow). See docs/updates.md.
import argparse
import os
import shutil
import shlex
import socket
import string
import subprocess
import sys
import tarfile
import tempfile
import time

import pexpect

OVMF_CODE = next((p for p in (
    "/usr/share/edk2/x64/OVMF_CODE.4m.fd",
    "/usr/share/edk2-ovmf/x64/OVMF_CODE.4m.fd",
    "/usr/share/OVMF/OVMF_CODE_4M.fd",
    "/usr/share/OVMF/OVMF_CODE.fd",
    "/usr/share/OVMF/OVMF_CODE.4m.fd",
    "/usr/share/OVMF/x64/OVMF_CODE.fd",
) if os.path.exists(p)), None)
OVMF_VARS = next((p for p in (
    "/usr/share/edk2/x64/OVMF_VARS.4m.fd",
    "/usr/share/edk2-ovmf/x64/OVMF_VARS.4m.fd",
    "/usr/share/OVMF/OVMF_VARS_4M.fd",
    "/usr/share/OVMF/OVMF_VARS.fd",
    "/usr/share/OVMF/OVMF_VARS.4m.fd",
    "/usr/share/OVMF/x64/OVMF_VARS.fd",
) if os.path.exists(p)), None)

# where a shipped ISO bakes the package closure (iso/build.sh); the TUI keys its
# RYOKU_ONLINE=0 decision off the same path (tui/system.go offlineRepoPath).
OFFLINE_REPO = "/usr/share/ryoku/offline/repo"

# the installed tree a real user must end up with: the package, the materialized
# config (including the files that used to reach no one), the bootloader, and the
# enabled greeter. checked by mounting the target root read-only after install.
INSTALLED_CHECKS = [
    ("d", "usr/share/ryoku/config"),
    ("f", "home/{user}/.config/quickshell/shell/shell.qml"),
    ("f", "home/{user}/.config/hypr/hyprland.lua"),
    ("f", "home/{user}/.config/pip/pip.conf"),
    ("f", "home/{user}/.config/wireplumber/wireplumber.conf.d/51-ryoku-bluetooth.conf"),
    # the default-app map ships in the site layer, never in ~/.config, which is
    # where the user's own "Set as default" picks live (see the PKGBUILD note).
    ("f", "usr/local/share/applications/mimeapps.list"),
    # and materialize must not recreate the user-owned one: that file getting
    # rewritten on every update is what threw away "Set as default" picks.
    ("!", "home/{user}/.config/mimeapps.list"),
    ("f", "home/{user}/.config/chromium-flags.conf"),
    ("f", "home/{user}/.config/chrome-flags.conf"),
    ("f", "usr/share/applications/ryoku-nvim.desktop"),
    ("d", "boot/EFI"),
]


def qemu_cmd(work, iso=None, with_iso=False, monitor=False):
    vars_copy = os.path.join(work, "OVMF_VARS.fd")
    if not os.path.exists(vars_copy):
        shutil.copy(OVMF_VARS, vars_copy)
    accel = ["-enable-kvm", "-cpu", "host"] if os.path.exists("/dev/kvm") else ["-cpu", "max"]
    cmd = [
        "qemu-system-x86_64", "-machine", "q35", *accel, "-m", "4096", "-smp", "4",
        "-drive", f"if=pflash,format=raw,readonly=on,file={OVMF_CODE}",
        "-drive", f"if=pflash,format=raw,file={vars_copy}",
        "-drive", f"file={os.path.join(work, 'target.qcow2')},if=virtio,format=qcow2",
        "-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0",
    ]
    if with_iso:
        cmd += ["-nographic", "-drive", f"file={iso},media=cdrom,readonly=on",
                "-boot", "d"]
    else:
        cmd += ["-display", "none", "-vga", "std", "-serial", "stdio"]
    if monitor:
        cmd += ["-monitor",
                f"unix:{os.path.join(work, 'mon.sock')},server,nowait"]
    return cmd


HMP_KEYS = {key: key for key in string.digits + string.ascii_lowercase}
HMP_KEYS["\n"] = "ret"


def spawn_qemu(cmd, timeout, log_path, mode):
    child = pexpect.spawn(cmd[0], cmd[1:], timeout=timeout, encoding="utf-8",
                          codec_errors="replace")
    child.logfile = open(log_path, mode)
    return child


def serial_tail(log_path):
    try:
        with open(log_path) as serial:
            return serial.read()[-4000:]
    except OSError as error:
        return f"(could not read {log_path}: {error})"


def hmp_command(work, command, timeout=10):
    monitor = os.path.join(work, "mon.sock")
    deadline = time.monotonic() + timeout
    last_error = None
    while time.monotonic() < deadline:
        try:
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
                client.settimeout(max(0.1, deadline - time.monotonic()))
                client.connect(monitor)
                response = b""
                while b"(qemu)" not in response:
                    chunk = client.recv(4096)
                    if not chunk:
                        raise RuntimeError("QEMU monitor closed before its prompt")
                    response += chunk
                client.sendall(command.encode() + b"\n")
                response = b""
                while b"(qemu)" not in response:
                    chunk = client.recv(4096)
                    if not chunk:
                        raise RuntimeError("QEMU monitor closed before command completion")
                    response += chunk
                text = response.decode("utf-8", errors="replace")
                if "Error:" in text:
                    raise RuntimeError(text.strip())
                return text
        except (FileNotFoundError, ConnectionRefusedError, socket.timeout) as error:
            last_error = error
            time.sleep(0.1)
    raise RuntimeError(f"QEMU monitor unavailable: {last_error}")


def enter_passphrase(work, passphrase, keypad):
    for character in passphrase:
        key = f"kp_{character}" if keypad else HMP_KEYS[character]
        hmp_command(work, f"sendkey {key}")
        time.sleep(0.15)
    hmp_command(work, f"sendkey {HMP_KEYS[chr(10)]}")


def convert_screendump(ppm):
    png = os.path.splitext(ppm)[0] + ".png"
    if shutil.which("magick"):
        converted = subprocess.run(["magick", ppm, png], check=False,
                                   stdout=subprocess.DEVNULL,
                                   stderr=subprocess.DEVNULL)
        if converted.returncode != 0:
            print(f"install-vm: magick could not convert {ppm}", file=sys.stderr)
        return
    try:
        from PIL import Image
    except ImportError:
        return
    try:
        with Image.open(ppm) as image:
            image.save(png)
    except Exception as error:
        print(f"install-vm: Pillow could not convert {ppm}: {error}", file=sys.stderr)


def capture_unlock_screen(work, name):
    ppm = os.path.join(work, f"unlock-{name}.ppm")
    hmp_command(work, f"screendump {ppm}")
    convert_screendump(ppm)

    captures = []
    for filename in os.listdir(work):
        if filename.startswith("unlock-") and filename.endswith(".ppm"):
            try:
                captures.append((int(filename[7:-4]), filename))
            except ValueError:
                pass
    later = sorted(item for item in captures if item[0] > 1)
    for _, filename in later[:-4]:
        for extension in (".ppm", ".png"):
            path = os.path.join(
                work, os.path.splitext(filename)[0] + extension)
            try:
                os.remove(path)
            except FileNotFoundError:
                pass


def screendumps(work):
    return sorted(
        os.path.join(work, name)
        for name in os.listdir(work)
        if name.startswith("unlock-") and name.endswith((".ppm", ".png"))
    )


def close_qemu(child):
    try:
        if child.logfile:
            child.logfile.flush()
            child.logfile.close()
            child.logfile = None
    except Exception:
        pass
    try:
        child.close(force=True)
    except Exception:
        pass


def fail_installed_boot(child, log_path, work, message):
    if child is not None:
        close_qemu(child)
    print(f"\ninstall-vm: {message}", file=sys.stderr)
    print(f"--- serial tail ---\n{serial_tail(log_path)}", file=sys.stderr)
    shots = screendumps(work)
    print("--- unlock screendumps ---", file=sys.stderr)
    if shots:
        for path in shots:
            print(f"{path} ({os.path.getsize(path)} bytes)", file=sys.stderr)
    else:
        print("(none)", file=sys.stderr)
    sys.exit(1)


def boot_installed(work, args, log_path):
    monitor = os.path.join(work, "mon.sock")
    try:
        os.remove(monitor)
    except FileNotFoundError:
        pass
    cmd = qemu_cmd(work, monitor=True)
    print("install-vm: booting the installed target disk")
    try:
        child = spawn_qemu(cmd, args.boot_timeout, log_path, "a")
    except OSError as error:
        fail_installed_boot(
            None, log_path, work, f"could not launch the installed boot: {error}")
    deadline = time.monotonic() + args.boot_timeout
    capture_number = 0
    try:
        if args.encrypt:
            wait = min(args.prompt_wait, max(0, deadline - time.monotonic()))
            result = child.expect([r"ryoku-test login:", pexpect.EOF,
                                   pexpect.TIMEOUT], timeout=wait)
            if result == 0:
                print("install-vm: installed system reached ryoku-test login:")
                close_qemu(child)
                return
            if result == 1:
                fail_installed_boot(child, log_path, work,
                                    "installed system exited before the unlock attempt")

            capture_unlock_screen(work, capture_number)
            capture_number += 1
            if args.wrong_first:
                enter_passphrase(
                    work, args.passphrase + "9", args.keypad)
                time.sleep(3)
                capture_unlock_screen(work, "wrong")
            enter_passphrase(work, args.passphrase, args.keypad)
            capture_unlock_screen(work, capture_number)
            capture_number += 1

        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                fail_installed_boot(
                    child, log_path, work,
                    f"installed system did not reach ryoku-test login: within "
                    f"{args.boot_timeout} seconds")
            wait = min(15, remaining) if args.encrypt else remaining
            result = child.expect([r"ryoku-test login:", pexpect.EOF,
                                   pexpect.TIMEOUT], timeout=wait)
            if result == 0:
                print("install-vm: installed system reached ryoku-test login:")
                close_qemu(child)
                return
            if result == 1:
                fail_installed_boot(child, log_path, work,
                                    "installed system exited before its login prompt")
            if args.encrypt:
                capture_unlock_screen(work, capture_number)
                capture_number += 1
    except (OSError, RuntimeError) as error:
        fail_installed_boot(child, log_path, work,
                            f"could not drive the installed boot: {error}")


def fail(child, msg):
    print(f"\ninstall-vm: {msg}", file=sys.stderr)
    try:
        child.close(force=True)
    except Exception:
        pass
    sys.exit(1)


def login(child):
    # archiso serial: an autologin root shell, or a login prompt (any hostname).
    i = child.expect([r"login:", r"# ", pexpect.TIMEOUT], timeout=300)
    if i == 0:
        child.sendline("root")
        if child.expect([r"Password:", r"# "], timeout=60) == 0:
            child.sendline("")
            child.expect(r"# ", timeout=60)
    elif i == 2:
        fail(child, "the live ISO never reached a serial prompt (see log)")


def sh(child, line, timeout=120):
    child.sendline(line)
    child.expect(r"# ", timeout=timeout)
    return child.before


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--iso", required=True)
    ap.add_argument("--work", default=None)
    ap.add_argument("--user", default="test")
    ap.add_argument("--profile", default="vm")
    ap.add_argument("--timeout", type=int, default=2400, help="install timeout (s)")
    ap.add_argument("--dry", action="store_true",
                    help="RYOKU_DRYRUN install-flow smoke (no real disk writes)")
    ap.add_argument("--boot-only", action="store_true", help="reach the live shell then stop")
    ap.add_argument("--repo-dir", default=None,
                    help="serve this [ryoku] repo (with <arch>/ryoku.db) to the "
                         "guest so the install pulls the desktop locally; needed in "
                         "CI, where Cloudflare 403s the public repo for datacenter IPs")
    ap.add_argument("--encrypt", action="store_true",
                    help="install the target with LUKS2 root encryption")
    ap.add_argument("--passphrase", default="123",
                    help="LUKS passphrase (default: 123)")
    ap.add_argument("--boot-installed", action="store_true",
                    help="boot the installed target and require its serial login prompt")
    ap.add_argument("--keypad", action="store_true",
                    help="type passphrase digits with QEMU keypad keys")
    ap.add_argument("--wrong-first", action="store_true",
                    help="try a wrong passphrase and capture the retry before unlocking")
    ap.add_argument("--prompt-wait", type=int, default=45,
                    help="seconds to wait before typing the LUKS passphrase (default: 45)")
    ap.add_argument("--boot-timeout", type=int, default=600,
                    help="installed-system boot timeout in seconds (default: 600)")
    ap.add_argument("--payload-dir", default=None,
                    help="overlay installation/backend and system from this checkout")
    args = ap.parse_args()
    if args.prompt_wait < 0:
        ap.error("--prompt-wait must not be negative")
    if args.boot_timeout <= 0:
        ap.error("--boot-timeout must be positive")
    if args.encrypt and not args.passphrase:
        ap.error("--passphrase must not be empty with --encrypt")
    if args.keypad:
        unsupported = sorted(set(args.passphrase) - set(string.digits))
        if unsupported:
            ap.error("--keypad requires an ASCII digits-only --passphrase "
                     f"(unsupported: {''.join(unsupported)!r})")
    if args.encrypt and args.boot_installed:
        unsupported = sorted(
            set(args.passphrase) - set(string.digits + string.ascii_lowercase))
        if unsupported:
            ap.error("--boot-installed can type only digits and lowercase letters "
                     f"in --passphrase (unsupported: {''.join(unsupported)!r})")

    if not OVMF_CODE or not OVMF_VARS:
        print("install-vm: OVMF firmware not found (pacman -S edk2-ovmf)", file=sys.stderr)
        sys.exit(2)

    work = args.work or tempfile.mkdtemp(prefix="ryoku-vm-")
    os.makedirs(work, exist_ok=True)
    log = os.path.join(work, "serial.log")
    subprocess.run(["qemu-img", "create", "-f", "qcow2",
                    os.path.join(work, "target.qcow2"), "40G"], check=True,
                   stdout=subprocess.DEVNULL)
    pwhash = subprocess.check_output(["openssl", "passwd", "-6", "test"]).decode().strip()
    servers = []
    repo_env = ""
    if args.repo_dir:
        if not os.path.exists(os.path.join(args.repo_dir, "x86_64", "ryoku.db")):
            print(f"install-vm: --repo-dir has no x86_64/ryoku.db ({args.repo_dir})",
                  file=sys.stderr)
            sys.exit(2)
        repo_srv = subprocess.Popen(
            ["python3", "-m", "http.server", "8710", "--bind", "127.0.0.1",
             "--directory", os.path.abspath(args.repo_dir)],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        servers.append(repo_srv)
        base = "http://10.0.2.2:8710"
        repo_env = (f" RYOKU_REPO_SERVER='{base}/$arch' RYOKU_REPO_SIGLEVEL=Never "
                    f"RYOKU_MIRROR_PROBE_URLS='https://geo.mirror.pkgbuild.com"
                    f"/core/os/x86_64/core.db {base}/x86_64/ryoku.db'")
        print(f"install-vm: serving [ryoku] repo from {args.repo_dir} at {base}")

    payload_url = None
    if args.payload_dir:
        payload_dir = os.path.abspath(args.payload_dir)
        payload_parts = [
            (os.path.join(payload_dir, "installation", "backend"),
             "installation/backend"),
            (os.path.join(payload_dir, "system"), "system"),
        ]
        missing_parts = [path for path, _ in payload_parts if not os.path.isdir(path)]
        if missing_parts:
            print("install-vm: --payload-dir is missing: " + ", ".join(missing_parts),
                  file=sys.stderr)
            sys.exit(2)
        payload_tar = os.path.join(work, "payload.tar")
        with tarfile.open(payload_tar, "w") as archive:
            for path, archive_name in payload_parts:
                archive.add(path, arcname=archive_name)
        payload_srv = subprocess.Popen(
            ["python3", "-m", "http.server", "8711", "--bind", "127.0.0.1",
             "--directory", work],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        servers.append(payload_srv)
        payload_url = "http://10.0.2.2:8711/payload.tar"
        print(f"install-vm: serving installer payload from {payload_dir} at "
              f"{payload_url}")
    print(f"install-vm: booting {args.iso} (KVM={'yes' if os.path.exists('/dev/kvm') else 'no, TCG'}); log -> {log}")
    child = spawn_qemu(qemu_cmd(work, args.iso, with_iso=True),
                       args.timeout, log, "w")
    try:
        login(child)
        print("install-vm: live shell reached")
        if args.boot_only:
            sh(child, "echo READY:$(uname -r)")
            child.sendline("poweroff")
            child.expect(pexpect.EOF, timeout=120)
            print("install-vm: boot-only check OK")
            return

        if payload_url:
            overlay = (
                f"curl -fsSL {shlex.quote(payload_url)} -o /tmp/ryoku-payload.tar"
                " && rm -rf /usr/share/ryoku/installation/backend"
                " /usr/share/ryoku/system"
                " && tar -xf /tmp/ryoku-payload.tar -C /usr/share/ryoku"
                " && grep -q '/usr/local/lib/ryoku/backend/ryoku-install'"
                " /usr/local/bin/ryoku-install"
                " && rm -rf /usr/local/lib/ryoku/backend"
                " && cp -a /usr/share/ryoku/installation/backend"
                " /usr/local/lib/ryoku/backend"
                " && test -x /usr/local/lib/ryoku/backend/ryoku-install"
                " && echo PAYLOAD_OVERLAY_OK"
            )
            if "PAYLOAD_OVERLAY_OK" not in sh(child, overlay, timeout=180):
                fail(child, "could not overlay the working-tree installer payload")
            print("install-vm: overlaid /usr/share/ryoku and "
                  "/usr/local/lib/ryoku/backend")

        env = (f"RYOKU_DISK=/dev/vda RYOKU_PROFILE={shlex.quote(args.profile)} "
               f"RYOKU_HOSTNAME=ryoku-test RYOKU_USERNAME={shlex.quote(args.user)} "
               f"RYOKU_DISK_STRATEGY=whole RYOKU_WIPE_CONFIRMED=1 RYOKU_SKIP_AUR=1 "
               f"RYOKU_COMPOSITOR=hyprland RYOKU_COMPOSITOR_CONFIG_DIR=hypr "
               f"RYOKU_BROWSER=chromium "
               f"RYOKU_REPO=/usr/share/ryoku RYOKU_KEYMAP=it RYOKU_XKB_LAYOUT=it "
               f"RYOKU_PASSWORD_HASH={shlex.quote(pwhash)}")
        if args.encrypt:
            env += (f" RYOKU_ENCRYPT=1 "
                    f"RYOKU_LUKS_PASSPHRASE={shlex.quote(args.passphrase)}")
        if args.dry:
            env += " RYOKU_DRYRUN=1"
        # Take the SAME package source the TUI would (system.go installEnv): a
        # shipped ISO bakes the whole closure, so the TUI always sends
        # RYOKU_ONLINE=0 and every real install is the offline path. Running the
        # backend without it exercised an online pacstrap no ISO user reaches, and
        # one that cannot work by construction: base.packages carries packages that
        # live only in [ryoku]/[offline], so pacstrap died on "target not found".
        baked = "R0E" in sh(
            child, f"ls {OFFLINE_REPO}/offline.db >/dev/null 2>&1; echo R$?E")
        if baked:
            env += f" RYOKU_ONLINE=0 RYOKU_OFFLINE_REPO={OFFLINE_REPO}"
            print("install-vm: offline path (the ISO's baked repo), as the TUI runs it")
        else:
            env += repo_env
            print("install-vm: online path (no baked repo on this ISO)")
        # skip the optional AUR builds (they compile from source for minutes and
        # are best-effort): RYOKU_SKIP_AUR covers a current ISO, emptying the set
        # also covers an older backend that predates the flag.
        sh(child, ": > /usr/share/ryoku/system/packages/aur.packages")
        # capability probe, BEFORE the install, and only meaningful on the online
        # path: an ISO whose backend predates RYOKU_REPO_SERVER cannot be pointed at
        # the locally served repo. This used to be inferred afterwards from
        # "repo.ryoku.dev" appearing anywhere in the serial log, which every real
        # failure also does (the backend prints that URL in its error text), so a
        # broken install reported success and the log was never even kept.
        stale_iso = "R0E" not in sh(
            child, "grep -rq RYOKU_REPO_SERVER /usr/share/ryoku/installation/backend; echo R$?E")
        child.sendline(f"export {env}; ryoku-install; echo BACKEND_EXIT:$?")
        i = child.expect([r"@@RYOKU_DONE", r"BACKEND_EXIT:[1-9]", pexpect.TIMEOUT],
                         timeout=args.timeout)
        if i != 0:
            if args.repo_dir and stale_iso and not baked:
                print("::warning::this ISO's backend predates RYOKU_REPO_SERVER; it "
                      "tried the public repo (Cloudflare 403s CI runners). Rebuild "
                      "the ISO for a full VM install -- skipping this run.",
                      file=sys.stderr)
                child.sendline("poweroff")
                try:
                    child.expect(pexpect.EOF, timeout=60)
                except (pexpect.TIMEOUT, pexpect.EOF):
                    pass
                return
            fail(child, "the installer did not reach @@RYOKU_DONE (see log)")
        child.expect(r"BACKEND_EXIT:0", timeout=120)
        child.expect(r"# ", timeout=60)
        print("install-vm: @@RYOKU_DONE, backend exit 0")
        if args.dry:
            child.sendline("poweroff")
            child.expect(pexpect.EOF, timeout=120)
            print("install-vm: dry-run install flow OK")
            return

        # Independent check: mount the installed tree (@ root, @home, ESP) and
        # assert it, so "the backend said done" is backed by real files on disk.
        sh(child, "umount -R /mnt 2>/dev/null; mkdir -p /mnt2")
        root_device = "/dev/vda2"
        verify_mapper_opened = False
        if args.encrypt:
            mapper = sh(
                child,
                "test -e /dev/mapper/root; echo VERIFY_ROOT_MAPPER_STATUS:$?")
            if "VERIFY_ROOT_MAPPER_STATUS:0" in mapper:
                root_device = "/dev/mapper/root"
            else:
                passphrase = shlex.quote(args.passphrase)
                opened = sh(
                    child,
                    f"printf '%s' {passphrase} | cryptsetup open --key-file=- "
                    "/dev/vda2 ryokuverify; echo VERIFY_OPEN_STATUS:$?",
                    timeout=120)
                if "VERIFY_OPEN_STATUS:0" not in opened:
                    print("install-vm: could not open the installed LUKS root:",
                          file=sys.stderr)
                    print(opened.strip(), file=sys.stderr)
                    fail(child, "encrypted-root verification failed")
                root_device = "/dev/mapper/ryokuverify"
                verify_mapper_opened = True
        mounted = sh(
            child,
            f"mount -o subvol=@ {root_device} /mnt2; "
            "echo VERIFY_MOUNT_STATUS:$?")
        if "VERIFY_MOUNT_STATUS:0" not in mounted:
            print(f"install-vm: could not mount {root_device} for verification:",
                  file=sys.stderr)
            print(mounted.strip(), file=sys.stderr)
            if verify_mapper_opened:
                sh(child, "cryptsetup close ryokuverify 2>/dev/null || true")
            fail(child, "installed-root verification mount failed")
        sh(child, f"mount -o subvol=@home {root_device} /mnt2/home 2>/dev/null || true")
        sh(child, "mount /dev/vda1 /mnt2/boot 2>/dev/null || true")
        missing = []
        for kind, rel in INSTALLED_CHECKS:
            path = "/mnt2/" + rel.format(user=args.user)
            # "!" asserts absence: some paths are a regression when they exist.
            if kind == "!":
                if "R0E" in sh(child, f"test -e {path}; echo R$?E"):
                    missing.append(f"must not exist:{path}")
                continue
            if "R0E" not in sh(child, f"test -{kind} {path}; echo R$?E"):
                missing.append(f"{kind}:{path}")
        if "enabled" not in sh(child, "systemctl --root=/mnt2 is-enabled sddm 2>&1"):
            missing.append("sddm not enabled (no greeter on boot)")
        for desc, cmd in [
            ("vconsole KEYMAP=it", "grep -q '^KEYMAP=it' /mnt2/etc/vconsole.conf && echo KBOK"),
            ("X11 XkbLayout it", "grep -q 'XkbLayout.*\"it\"' /mnt2/etc/X11/xorg.conf.d/00-keyboard.conf && echo KBOK"),
            ("hypr kb_layout it", f"grep -q 'kb_layout = \"it\"' /mnt2/home/{args.user}/.config/hypr/keyboard.lua && echo KBOK"),
        ]:
            if "KBOK" not in sh(child, cmd):
                missing.append("keymap: " + desc)
        if args.encrypt:
            crypttab = sh(
                child,
                "test -f /mnt2/etc/crypttab; echo CRYPTTAB_STATUS:$?")
            if "CRYPTTAB_STATUS:0" not in crypttab:
                missing.append("encrypted install has no /etc/crypttab")
            cryptdevice = sh(
                child,
                "grep -R --include='*.conf' -q 'cryptdevice=' /mnt2/boot; "
                "echo CRYPTDEVICE_STATUS:$?")
            if "CRYPTDEVICE_STATUS:0" not in cryptdevice:
                missing.append("Limine config has no cryptdevice= kernel argument")
        if args.boot_installed:
            enabled = sh(
                child,
                "systemctl --root=/mnt2 enable serial-getty@ttyS0.service "
                ">/dev/null; echo SERIAL_GETTY_STATUS:$?")
            if "SERIAL_GETTY_STATUS:0" not in enabled:
                missing.append("serial-getty@ttyS0.service could not be enabled")
        sh(child, "umount -R /mnt2 2>/dev/null || true")
        if verify_mapper_opened:
            sh(child, "cryptsetup close ryokuverify")
        child.sendline("poweroff")
        child.expect(pexpect.EOF, timeout=120)
        if child.logfile:
            child.logfile.flush()
            child.logfile.close()
            child.logfile = None
        if missing:
            print("install-vm: installed tree is missing:", file=sys.stderr)
            for item in missing:
                print(f"  {item}", file=sys.stderr)
            sys.exit(1)
        print("install-vm: installed tree verified")
        if args.boot_installed:
            boot_installed(work, args, log)
    except (pexpect.TIMEOUT, pexpect.EOF) as error:
        try:
            if child.logfile:
                child.logfile.flush()
        except Exception:
            pass
        print(f"\ninstall-vm: {type(error).__name__}\n--- serial tail ---\n"
              f"{serial_tail(log)}", file=sys.stderr)
        sys.exit(1)
    finally:
        for server in servers:
            server.terminate()
        for server in servers:
            try:
                server.wait(timeout=5)
            except subprocess.TimeoutExpired:
                server.kill()


if __name__ == "__main__":
    main()
