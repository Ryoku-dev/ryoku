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
import subprocess
import sys
import tempfile

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


def qemu_cmd(work, iso, with_iso):
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
        "-nographic",
    ]
    if with_iso:
        cmd += ["-drive", f"file={iso},media=cdrom,readonly=on", "-boot", "d"]
    return cmd


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
    args = ap.parse_args()

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
    repo_srv = None
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
        base = "http://10.0.2.2:8710"  # qemu's user-mode gateway -> the host loopback
        repo_env = (f" RYOKU_REPO_SERVER='{base}/$arch' RYOKU_REPO_SIGLEVEL=Never "
                    f"RYOKU_MIRROR_PROBE_URLS='https://geo.mirror.pkgbuild.com"
                    f"/core/os/x86_64/core.db {base}/x86_64/ryoku.db'")
        print(f"install-vm: serving [ryoku] repo from {args.repo_dir} at {base}")

    print(f"install-vm: booting {args.iso} (KVM={'yes' if os.path.exists('/dev/kvm') else 'no, TCG'}); log -> {log}")
    child = pexpect.spawn(" ".join(qemu_cmd(work, args.iso, with_iso=True)),
                          timeout=args.timeout, encoding="utf-8", codec_errors="replace")
    child.logfile = open(log, "w")
    try:
        login(child)
        print("install-vm: live shell reached")
        if args.boot_only:
            sh(child, "echo READY:$(uname -r)")
            child.sendline("poweroff")
            child.expect(pexpect.EOF, timeout=120)
            print("install-vm: boot-only check OK")
            return

        env = (f"RYOKU_DISK=/dev/vda RYOKU_PROFILE={args.profile} "
               f"RYOKU_HOSTNAME=ryoku-test RYOKU_USERNAME={args.user} "
               f"RYOKU_DISK_STRATEGY=whole RYOKU_WIPE_CONFIRMED=1 RYOKU_SKIP_AUR=1 "
               f"RYOKU_COMPOSITOR=hyprland RYOKU_COMPOSITOR_CONFIG_DIR=hypr "
               f"RYOKU_BROWSER=chromium "
               f"RYOKU_REPO=/usr/share/ryoku RYOKU_KEYMAP=it RYOKU_XKB_LAYOUT=it RYOKU_PASSWORD_HASH='{pwhash}'")
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

        # independent check: mount the installed tree (@ root, @home, ESP) and
        # assert it, so "the backend said done" is backed by real files on disk.
        sh(child, "umount -R /mnt 2>/dev/null; mkdir -p /mnt2")
        sh(child, "mount -o subvol=@ /dev/vda2 /mnt2")
        sh(child, "mount -o subvol=@home /dev/vda2 /mnt2/home 2>/dev/null || true")
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
        sh(child, "umount -R /mnt2 2>/dev/null || true")
        child.sendline("poweroff")
        child.expect(pexpect.EOF, timeout=120)
        if missing:
            print("install-vm: installed tree is missing:", file=sys.stderr)
            for m in missing:
                print(f"  {m}", file=sys.stderr)
            sys.exit(1)
        print("install-vm: installed tree verified")
    except (pexpect.TIMEOUT, pexpect.EOF) as e:
        with open(log) as f:
            tail = f.read()[-4000:]
        print(f"\ninstall-vm: {type(e).__name__}\n--- serial tail ---\n{tail}", file=sys.stderr)
        sys.exit(1)
    finally:
        if repo_srv:
            repo_srv.terminate()


if __name__ == "__main__":
    main()
