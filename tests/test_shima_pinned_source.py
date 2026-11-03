import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
OVERLAY = ROOT / "ryoku/shell/quickshell/shima-overlay"
SCRIPTS = ROOT / "ryoku/shell/scripts"
ARCHIVE = os.environ.get("SHIMA_TEST_ARCHIVE")


class ShimaPinnedSourceTest(unittest.TestCase):
    def test_source_and_runtime_integrity(self):
        meta = json.loads((OVERLAY / "upstream.json").read_text())
        self.assertEqual(meta["version"], "2.33.0")
        self.assertEqual(meta["runtime_files"], 2409)
        self.assertEqual(len(meta["revision"]), 40)
        self.assertEqual(hashlib.sha256((OVERLAY / "runtime.patch").read_bytes()).hexdigest(), meta["patch_sha256"])
        self.assertEqual(hashlib.sha256((OVERLAY / "shima-overview-terminal.patch").read_bytes()).hexdigest(), meta["overview_patch_sha256"])

    def test_legacy_shell_remains_untouched(self):
        self.assertFalse((ROOT / "ryoku/shell/quickshell/shima").exists())
        self.assertTrue((ROOT / "ryoku/shell/quickshell/inir/ShellIrisPanels.qml").is_file())
        self.assertTrue((ROOT / "ryoku/shell/quickshell/shell/modules/bar/barstyles/iris/Scene.qml").is_file())

    def test_upstream_services_are_not_installed(self):
        files = (OVERLAY / "runtime-files.txt").read_text().splitlines()
        self.assertEqual(len(files), 2403)
        self.assertEqual(len(set(files)), 2403)
        self.assertFalse(any(name.endswith(".service") for name in files))
        self.assertTrue((SCRIPTS / "ryoku-shima-install").is_file())

    def test_supervisor_falls_back_without_opt_in(self):
        code = (ROOT / "ryoku/shell/ipc/shima.go").read_text()
        self.assertIn('os.Getenv("RYOKU_SHIMA_UPSTREAM") != "1"', code)
        self.assertIn("fallbackShima(", code)
        self.assertIn("terminateDesktop(", code)

    def test_required_integration_patches(self):
        patch = (OVERLAY / "runtime.patch").read_text()
        for expected in ("RyokuHost.qml", "RyokuTokens.qml", "RyokuVisualizerPalette.qml",
                         "modules/pill/PillSettings.qml", "modules/overview/OverviewNiriWidget.qml",
                         'id: ryokuLauncher; target: "launcher"'):
            self.assertIn(expected, patch)

    @unittest.skipUnless(ARCHIVE, "Set SHIMA_TEST_ARCHIVE for source installation")
    def test_verified_offline_install(self):
        with tempfile.TemporaryDirectory(prefix="shima-pinned-test-") as directory:
            root = Path(directory)
            env = os.environ.copy()
            for key in ("HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"):
                folder = root / key.lower()
                folder.mkdir()
                env[key] = str(folder)
            config = root / "xdg_config_home/quickshell/shima-overlay"
            config.parent.mkdir()
            shutil.copytree(OVERLAY, config)
            installer = str(SCRIPTS / "ryoku-shima-install")
            def run(*args):
                return subprocess.run([installer, *args], env=env, capture_output=True, text=True, timeout=90)
            bad = root / "invalid.tar.gz"
            bad.write_bytes(b"invalid archive bytes")
            result = run("install", "--archive", str(bad))
            self.assertNotEqual(result.returncode, 0)
            current = root / "xdg_data_home/ryoku/shima/current"
            self.assertFalse(current.exists())
            result = run("install", "--archive", ARCHIVE)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(run("verify").returncode, 0)
            self.assertTrue((current / "shell.qml").is_file())
            self.assertTrue((current / "ShellWafflePanels.qml").is_file())
            self.assertTrue((current / "ShimaOverviewBackdrop.qml").is_file())
            self.assertIn("overview_patch_sha256", json.loads((current / "runtime.json").read_text()))
            self.assertIn("ryoku-overview-backdrop", (current / "ShimaOverviewBackdrop.qml").read_text())
            self.assertIn("property bool enableTerminal: false", (current / "modules/common/Config.qml").read_text())
            self.assertFalse((current / "assets/systemd/inir.service").exists())
            launcher = subprocess.run([str(SCRIPTS / "ryoku-shima"), "--version"], env=env, capture_output=True, text=True)
            self.assertEqual(launcher.returncode, 0, launcher.stderr)
            self.assertIn("2.33.0", launcher.stdout)
            self.assertNotEqual(subprocess.run([str(SCRIPTS / "ryoku-shima")], env=env, capture_output=True).returncode, 0)
            env["https_proxy"] = "http://127.0.0.1:1"
            self.assertEqual(run("install").returncode, 0)
            self.assertEqual(run("verify").returncode, 0)


if __name__ == "__main__":
    unittest.main()
