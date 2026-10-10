#!/usr/bin/env python3
"""Install the Ryoku Void ISO over serial, then require a booted login prompt."""
import argparse
import os
import shlex
import shutil
import subprocess
import sys
import tempfile

import pexpect


def first_existing(paths):
    return next((path for path in paths if os.path.exists(path)), None)


OVMF_CODE = first_existing((
    "/usr/share/edk2/x64/OVMF_CODE.4m.fd",
    "/usr/share/edk2-ovmf/x64/OVMF_CODE.4m.fd",
    "/usr/share/OVMF/OVMF_CODE_4M.fd",
    "/usr/share/OVMF/OVMF_CODE.fd",
))
OVMF_VARS = first_existing((
    "/usr/share/edk2/x64/OVMF_VARS.4m.fd",
    "/usr/share/edk2-ovmf/x64/OVMF_VARS.4m.fd",
    "/usr/share/OVMF/OVMF_VARS_4M.fd",
    "/usr/share/OVMF/OVMF_VARS.fd",
))
OFFLINE_REPO = "/run/initramfs/live/ryoku/repo"
LIVE_CSI_PREFIX = r"(?:\x1b\[[0-9;?]*[A-Za-z])*"
LIVE_LOGIN_RE = rf"(?m)^{LIVE_CSI_PREFIX}[^\r\n]* login: ?$"
LIVE_SHELL_RE = rf"(?m)^{LIVE_CSI_PREFIX}-?bash-[0-9][0-9.]*# $"
LIVE_PROMPT_RE = rf"(?m)^{LIVE_CSI_PREFIX}RYOKU_LIVE# $"
LIVE_SYNC_RE = rf"(?m)^{LIVE_CSI_PREFIX}RYOKU_SYNC_42\r*$"


def qemu_command(work, iso=None):
    vars_copy = os.path.join(work, "OVMF_VARS.fd")
    if not os.path.exists(vars_copy):
        shutil.copy2(OVMF_VARS, vars_copy)
    accel = ["-enable-kvm", "-cpu", "host"] if os.path.exists("/dev/kvm") else ["-cpu", "max"]
    command = [
        "qemu-system-x86_64", "-machine", "q35", *accel,
        "-m", "4096", "-smp", "4",
        "-drive", f"if=pflash,format=raw,readonly=on,file={OVMF_CODE}",
        "-drive", f"if=pflash,format=raw,file={vars_copy}",
        "-drive", f"file={os.path.join(work, 'target.qcow2')},if=virtio,format=qcow2",
        "-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0",
        "-nographic",
    ]
    if iso:
        command += ["-drive", f"file={os.path.abspath(iso)},media=cdrom,readonly=on", "-boot", "d"]
    else:
        command += ["-boot", "c"]
    return command


def serial_tail(path):
    try:
        with open(path, encoding="utf-8", errors="replace") as stream:
            return stream.read()[-5000:]
    except OSError as error:
        return f"could not read serial log: {error}"


def spawn(command, timeout, log_path, mode):
    child = pexpect.spawn(command[0], command[1:], encoding="utf-8",
                          codec_errors="replace", timeout=timeout)
    child.logfile = open(log_path, mode, encoding="utf-8")
    return child


def _reach_live_shell(child, timeout):
    choice = child.expect(
        [LIVE_LOGIN_RE, LIVE_SHELL_RE, LIVE_PROMPT_RE], timeout=timeout)
    if choice != 0:
        return
    automatic = child.expect(
        [r"\(automatic login\)", pexpect.TIMEOUT], timeout=2)
    if automatic == 0:
        child.expect(LIVE_SHELL_RE, timeout=20)
        return
    child.sendline("root")
    choice = child.expect([r"(?m)^Password: ?$", LIVE_SHELL_RE], timeout=20)
    if choice == 0:
        child.sendline("voidlinux")
        child.expect(LIVE_SHELL_RE, timeout=20)


def reach_live_root(child):
    sync_command = (
        "bind 'set enable-bracketed-paste off' 2>/dev/null; "
        "export PS1='RYOKU_LIVE# '; echo RYOKU_SYNC_$((40+2))"
    )
    for attempt in range(6):
        try:
            if attempt:
                # A respawned getty may be sitting silently at login or at a
                # fresh shell prompt. A blank line makes either state visible.
                child.sendline("")
            _reach_live_shell(child, 300 if attempt == 0 else 20)
            child.sendline(sync_command)
            child.expect(LIVE_SYNC_RE, timeout=15)
            child.expect(LIVE_PROMPT_RE, timeout=15)
            return
        except pexpect.TIMEOUT:
            continue
        except pexpect.EOF as error:
            raise RuntimeError(
                "live serial getty exited during prompt sync") from error
    raise RuntimeError("live ISO did not produce a stable serial root shell")


def shutdown(child):
    child.sendline("poweroff")
    try:
        child.expect(pexpect.EOF, timeout=180)
    finally:
        child.close(force=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--iso", required=True, help="Ryoku Void ISO")
    parser.add_argument("--work", help="persistent work directory")
    parser.add_argument("--timeout", type=int, default=3600,
                        help="backend timeout in seconds (default: 3600)")
    parser.add_argument("--boot-timeout", type=int, default=600,
                        help="installed boot timeout in seconds (default: 600)")
    args = parser.parse_args()

    if not OVMF_CODE or not OVMF_VARS:
        parser.error("OVMF firmware was not found")
    for command in ("qemu-system-x86_64", "qemu-img", "openssl"):
        if not shutil.which(command):
            parser.error(f"required command not found: {command}")

    work = os.path.abspath(args.work or tempfile.mkdtemp(prefix="ryoku-void-vm-"))
    os.makedirs(work, exist_ok=True)
    vars_copy = os.path.join(work, "OVMF_VARS.fd")
    if os.path.exists(vars_copy):
        os.remove(vars_copy)
    disk = os.path.join(work, "target.qcow2")
    if os.path.exists(disk):
        os.remove(disk)
    subprocess.run(["qemu-img", "create", "-f", "qcow2", disk, "40G"], check=True)
    password_hash = subprocess.check_output(
        ["openssl", "passwd", "-6", "test"], text=True).strip()
    log_path = os.path.join(work, "serial.log")

    child = spawn(qemu_command(work, args.iso), args.timeout, log_path, "w")
    try:
        reach_live_root(child)
        child.sendline(
            f"test -r {OFFLINE_REPO}/x86_64-repodata && "
            f"ls {OFFLINE_REPO}/*.xbps >/dev/null 2>&1; echo REPO_READY:$?")
        child.expect(r"REPO_READY:0", timeout=60)
        child.expect(LIVE_PROMPT_RE, timeout=60)

        environment = {
            "RYOKU_DISK": "/dev/vda",
            "RYOKU_DISK_STRATEGY": "whole",
            "RYOKU_WIPE_CONFIRMED": "1",
            "RYOKU_HOSTNAME": "ryoku-test",
            "RYOKU_USERNAME": "test",
            "RYOKU_PASSWORD_HASH": password_hash,
            "RYOKU_COMPOSITOR": "niri",
            "RYOKU_COMPOSITOR_CONFIG_DIR": "niri",
            "RYOKU_BROWSER": "firefox",
            "RYOKU_LOGIN_SHELL": "fish",
            "RYOKU_PROFILE": "vm",
            "RYOKU_ONLINE": "0",
            "RYOKU_OFFLINE_REPO": OFFLINE_REPO,
            "RYOKU_REPO": "/usr/share/ryoku",
            "RYOKU_ENABLE_SERIAL": "1",
            "RYOKU_KERNEL_CMDLINE_EXTRA": "console=ttyS0,115200n8",
        }
        exports = " ".join(
            f"{name}={shlex.quote(value)}" for name, value in environment.items())
        child.sendline(f"export {exports}; ryoku-install; echo BACKEND_EXIT:$?")
        result = child.expect(
            [r"@@RYOKU_DONE", r"BACKEND_EXIT:[1-9][0-9]*", pexpect.TIMEOUT],
            timeout=args.timeout)
        if result != 0:
            raise RuntimeError("backend did not emit @@RYOKU_DONE")
        child.expect(r"BACKEND_EXIT:0", timeout=180)
        child.expect(LIVE_PROMPT_RE, timeout=60)
        shutdown(child)
    except (pexpect.TIMEOUT, pexpect.EOF, RuntimeError) as error:
        child.close(force=True)
        print(f"install-vm-void: {error}\n--- serial tail ---\n{serial_tail(log_path)}",
              file=sys.stderr)
        return 1

    boot_log = os.path.join(work, "installed-serial.log")
    installed = spawn(qemu_command(work), args.boot_timeout, boot_log, "w")
    try:
        installed.expect(r"(?m)^ryoku-test login: ?$", timeout=args.boot_timeout)
        print("install-vm-void: installed Void system reached the serial login prompt")
        installed.close(force=True)
    except (pexpect.TIMEOUT, pexpect.EOF) as error:
        installed.close(force=True)
        print(f"install-vm-void: installed boot failed: {error}\n--- serial tail ---\n"
              f"{serial_tail(boot_log)}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
