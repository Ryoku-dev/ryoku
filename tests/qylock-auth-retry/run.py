#!/usr/bin/env python3
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def main() -> int:
    repo = Path(__file__).resolve().parents[2]
    lock_shell = repo / "ryoku/lockscreen/qylock/quickshell-lockscreen/lock_shell.qml"
    host_source = lock_shell.read_text(encoding="utf-8")
    if "retryAllowed: sddmShim.authRetryInputAllowed" not in host_source:
        print("qylock-auth-retry: production host does not use the dedicated retry gate", file=sys.stderr)
        return 1
    tests = Path(__file__).resolve().parent / "Tests"
    fixtures = Path(__file__).resolve().parent / "imports"
    home = Path(tempfile.mkdtemp(prefix="qylock-auth-retry-"))
    try:
        config = home / "config" / "qylock"
        config.mkdir(parents=True)
        (config / "auth-retry").write_text("on\n", encoding="utf-8")
        runtime = home / "runtime"
        runtime.mkdir()
        os.environ.update({
            "HOME": str(home),
            "XDG_CONFIG_HOME": str(home / "config"),
            "XDG_RUNTIME_DIR": str(runtime),
            "DBUS_SESSION_BUS_ADDRESS": "unix:path=/nonexistent",
            "DBUS_SYSTEM_BUS_ADDRESS": "unix:path=/nonexistent",
            "QT_QPA_PLATFORM": "offscreen",
            "QT_QUICK_BACKEND": "software",
            "QSG_RHI_BACKEND": "software",
            "QML2_IMPORT_PATH": str(fixtures),
            "QML_IMPORT_PATH": str(fixtures),
            "QYLOCK_TEST_REPO": str(repo),
        })
        if shutil.which("qmltestrunner"):
            return subprocess.run(
                ["qmltestrunner", "-input", str(tests), "-import", str(fixtures), *sys.argv[1:]],
                check=False,
            ).returncode
        try:
            from PySide6.QtQuickTest import QUICK_TEST_MAIN
        except ImportError:
            print("qylock-auth-retry: install Qt qmltestrunner or PySide6", file=sys.stderr)
            return 77
        return QUICK_TEST_MAIN("qylock-auth-retry", [sys.argv[0], "-input", str(tests), "-import", str(fixtures), *sys.argv[1:]])
    finally:
        shutil.rmtree(home, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
