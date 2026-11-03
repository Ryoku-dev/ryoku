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

    def test_ricelin_switcher_keeps_all_family_and_bar_choices(self):
        qml = (RUNTIME / "modules/pill/PillSettings.qml").read_text()
        self.assertIn('text: Translation.tr("Switch shell & bar")', qml)
        self.assertIn('root.switcherOpen = !root.switcherOpen', qml)
        self.assertIn('Config.setNestedValue("bar.appearanceStyle", style)', qml)
        self.assertIn('Quickshell.shellPath("scripts/inir"), "panelFamily", "set", family', qml)
        for family in ('"iris"', '"ii"', '"waffle"'):
            self.assertIn('id: ' + family, qml)
        for style in ('"classic"', '"islands"', '"scenic"', '"frame"', '"m3"', '"pill"'):
            self.assertIn('id: ' + style, qml)

    def test_default_overview_uses_ryogami_wallpaper(self):
        qml = (RUNTIME / "modules/overview/OverviewNiriWidget.qml").read_text()
        self.assertIn('RyokuHost.ready ? RyokuHost.pathFor(monitor)', qml)
        self.assertIn('Wallpapers.stillUrlFor(hostPath)', qml)
        self.assertIn('WallpaperListener.wallpaperUrlForScreen', qml)
        self.assertIn('source: root.wallpaperPath', qml)

    def test_launcher_is_distinct_from_overview(self):
        qml = (RUNTIME / "shell.qml").read_text()
        self.assertIn('case "launcher": return ryokuLauncher', qml)
        self.assertIn('case "overview": return ryokuOverview', qml)
        self.assertIn('id: ryokuLauncher; target: "launcher"', qml)
        self.assertIn('GlobalStates.pillSurfaceCommand("open", "launcher")', qml)
        self.assertIn('GlobalStates.searchOpen = true', qml)
        self.assertIn('GlobalStates.openOverview("")', qml)

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
