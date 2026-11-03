import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
RUNTIME = ROOT / "ryoku/shell/quickshell/shima"
SHELL = ROOT / "ryoku/shell/quickshell/shell"
LEGACY = ROOT / "ryoku/shell/quickshell/inir"
LAUNCHER = ROOT / "ryoku/shell/scripts/ryoku-shima"


class ShimaUpstreamParityTest(unittest.TestCase):
    def test_existing_native_shima_remains_available(self):
        self.assertTrue((LEGACY / "ShellIrisPanels.qml").is_file())
        self.assertTrue((SHELL / "modules/bar/barstyles/iris/Scene.qml").is_file())

    def test_complete_pinned_runtime(self):
        self.assertEqual((RUNTIME / "VERSION").read_text().strip(), "2.33.0")
        self.assertTrue((RUNTIME / "LICENSE").is_file())
        self.assertTrue((RUNTIME / "NOTICE").is_file())
        self.assertTrue((RUNTIME / "shell.qml").is_file())
        self.assertGreaterEqual(len(list(RUNTIME.rglob("*.qml"))), 1400)
        for filename in ("ShellIrisPanels.qml", "ShellIiPanels.qml", "ShellWafflePanels.qml"):
            with self.subTest(filename=filename):
                self.assertTrue((RUNTIME / filename).is_file())

    def test_ryoku_integrations_are_included(self):
        for filename in (
            "services/RyokuTokens.qml",
            "services/RyokuVisualizerPalette.qml",
            "services/RyokuHost.qml",
            "services/PolkitServiceImpl.qml",
            "modules/iris/frame/IrisFrame.qml",
            "modules/iris/settings/IrisOptions.qml",
            "modules/background/widgets/OrganicEdgeWidget.qml",
        ):
            with self.subTest(filename=filename):
                self.assertTrue((RUNTIME / filename).is_file())

    def test_symlink_targets_stay_within_bundle(self):
        root = RUNTIME.resolve()
        for link in RUNTIME.rglob("*"):
            if link.is_symlink():
                self.assertTrue(link.resolve().is_relative_to(root), str(link))

    def test_arch_launcher_is_managed_and_portable(self):
        with tempfile.TemporaryDirectory(prefix="ryoku-shima-check-") as tmp:
            env = os.environ.copy()
            env["HOME"] = tmp
            env["XDG_CONFIG_HOME"] = str(Path(tmp) / "config")
            env["RYOKU_SHIMA_RUNTIME_DIR"] = str(RUNTIME)
            env.pop("RYOKU_SHIMA_MANAGED", None)
            proc = subprocess.run([str(LAUNCHER), "--root"], env=env, capture_output=True, text=True)
            self.assertEqual(proc.returncode, 0, proc.stderr)
            self.assertEqual(proc.stdout.strip(), str(RUNTIME))
            version = subprocess.run([str(LAUNCHER), "--version"], env=env, capture_output=True, text=True)
            self.assertEqual(version.returncode, 0, version.stderr)
            self.assertIn("2.33.0", version.stdout)
            unmanaged = subprocess.run([str(LAUNCHER)], env=env, capture_output=True, text=True)
            self.assertNotEqual(unmanaged.returncode, 0)
            self.assertIn("supervised by ryoku-shell", unmanaged.stderr)
            self.assertFalse((Path(tmp) / "config/ryoku/shima").exists())

    def test_bundled_script_interpreters_are_portable(self):
        import subprocess
        for path in RUNTIME.rglob("*"):
            if not path.is_file() or path.is_symlink():
                continue
            with path.open("rb") as stream:
                first_line = stream.readline()
            if first_line.startswith(b"#!"):
                self.assertNotIn(b"/nix/store/", first_line, str(path))
                self.assertNotIn(b"INIR_VENV", first_line, str(path))
        for filename in ("scripts/inir", "scripts/restart-shell.sh",
                         "scripts/cava/generate_config.sh"):
            path = RUNTIME / filename
            self.assertEqual(
                subprocess.run(["bash", "-n", str(path)], capture_output=True).returncode,
                0, str(path),
            )

    def test_required_arch_dependencies(self):
        packages = (ROOT / "system/packages/base.packages").read_text().splitlines()
        for package in ("quickshell", "cava", "matugen", "qt6-imageformats",
                        "qt6-5compat", "qt6-multimedia", "kirigami", "syntax-highlighting"):
            with self.subTest(package=package):
                self.assertIn(package, packages)

    def test_legacy_default_is_explicitly_opt_in(self):
        source = (ROOT / "ryoku/shell/ipc/shima.go").read_text()
        self.assertIn('os.Getenv("RYOKU_SHIMA_UPSTREAM") != "1"', source)
        self.assertIn('os.Getenv("NIRI_SOCKET") == ""', source)
        self.assertIn('d.fallbackShima(', source)
        self.assertIn('terminateDesktop(', source)


if __name__ == "__main__":
    unittest.main()
