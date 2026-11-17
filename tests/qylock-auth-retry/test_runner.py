#!/usr/bin/env python3
"""Check the native runner subprocess boundary without Qt or authentication."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class NativeRunnerCleanupTests(unittest.TestCase):
    def check_native_exit(self, exit_code):
        with tempfile.TemporaryDirectory(prefix="qylock-runner-test-") as directory:
            root = Path(directory)
            binaries = root / "bin"
            binaries.mkdir()
            scratch = root / "scratch"
            scratch.mkdir()
            marker = root / "observed-home"
            native = binaries / "qmltestrunner"
            native.write_text(
                '#!/bin/sh\nprintf "%s" "$HOME" > "$QYLOCK_TEST_HOME_MARKER"\n'
                'exit "$QYLOCK_TEST_EXIT"\n', encoding="utf-8"
            )
            native.chmod(0o700)
            environment = os.environ.copy()
            environment.update({
                "PATH": str(binaries) + os.pathsep + os.defpath,
                "TMPDIR": str(scratch),
                "QYLOCK_TEST_HOME_MARKER": str(marker),
                "QYLOCK_TEST_EXIT": str(exit_code),
            })
            result = subprocess.run(
                [sys.executable, str(Path(__file__).with_name("run.py")), "-v1"],
                cwd=root, env=environment, capture_output=True, text=True,
                timeout=15, check=False,
            )
            self.assertEqual(result.returncode, exit_code, result.stderr)
            observed_home = Path(marker.read_text(encoding="utf-8"))
            self.assertEqual(observed_home.parent, scratch)
            self.assertTrue(observed_home.name.startswith("qylock-auth-retry-"))
            self.assertFalse(observed_home.exists(), "runner left its isolated HOME behind")
            self.assertEqual(list(scratch.iterdir()), [])

    def test_native_success_cleans_temporary_home(self):
        self.check_native_exit(0)

    def test_native_failure_cleans_temporary_home(self):
        self.check_native_exit(7)


if __name__ == "__main__":
    unittest.main()
