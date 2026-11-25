#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Run the Oni asset, mark and attribution checks against injected violations."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parent.parent
_spec = importlib.util.spec_from_file_location("hyakki_native", ROOT / "release/native.py")
native = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(native)

COPIED = ("NOTICE", "release/oni-assets.json", *native.ONI_TEXT)
CHARACTER = "internal/oni/characters/oni"
# Built from escapes so this file never carries the marks it tests for.
MARKS = ("N\u016bs", "Nu\u0304s", "N\u016aS", "Kn" + "ot", "Ni" + "xi", "hey" + "nixi", "Cou" + "cou")


class OniLegalTest(unittest.TestCase):
    def setUp(self):
        self._dirs = []

    def tearDown(self):
        for directory in self._dirs:
            directory.cleanup()

    def copy(self):
        directory = tempfile.TemporaryDirectory(prefix="hyakki-oni-legal-")
        self._dirs.append(directory)
        root = Path(directory.name)
        for name in COPIED:
            source, target = ROOT / name, root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            if source.is_dir():
                shutil.copytree(source, target, symlinks=True)
            elif source.is_file():
                shutil.copyfile(source, target)
        return root

    def manifest(self, root):
        return json.loads((root / native.ONI_ASSETS).read_text(encoding="utf-8"))

    def write_manifest(self, root, data):
        data["assets"].sort(key=lambda entry: entry["path"])
        (root / native.ONI_ASSETS).write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")

    def append(self, path, text):
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("a", encoding="utf-8") as handle:
            handle.write(text)

    def fails(self, root, reason, record=False):
        with self.assertRaises(ValueError) as caught:
            if record:
                native.record_oni_assets(root)
            else:
                native.check_oni_assets(root)
        self.assertIn(reason, str(caught.exception))
        return str(caught.exception)

    def test_clean_copy_passes(self):
        native.check_oni_assets(self.copy())

    def test_embed_code_is_not_an_asset(self):
        root = self.copy()
        self.append(root / native.ONI_CHARACTERS / "more.go", "package characters\n")
        native.check_oni_assets(root)

    def test_undeclared_character_asset(self):
        root = self.copy()
        (root / CHARACTER / "extra.png").write_bytes(b"\x89PNG\r\n")
        self.fails(root, "undeclared character asset: internal/oni/characters/oni/extra.png")

    def test_second_character_directory_must_be_declared(self):
        root = self.copy()
        self.append(root / native.ONI_CHARACTERS / "moth/profile.json", "{}\n")
        self.fails(root, "undeclared character asset")

    def test_symlinked_character_asset(self):
        root = self.copy()
        os.symlink("persona.md", root / CHARACTER / "alias.md")
        self.fails(root, "symlink in shipped character assets")

    def test_missing_license_file(self):
        root = self.copy()
        (root / CHARACTER / "LICENSE.artwork").unlink()
        self.fails(root, "declared license file is missing")

    def test_license_file_pointing_nowhere(self):
        root = self.copy()
        data = self.manifest(root)
        data["assets"][-1]["license_file"] = f"{CHARACTER}/LICENSE.absent"
        self.write_manifest(root, data)
        self.fails(root, "declared license file is missing")

    def test_license_file_outside_notice_inventory(self):
        root = self.copy()
        data = self.manifest(root)
        data["assets"][-1]["license_file"] = f"{CHARACTER}/PROVENANCE.md"
        self.write_manifest(root, data)
        self.fails(root, "not collected into THIRD_PARTY_NOTICES")

    def test_missing_provenance(self):
        root = self.copy()
        (root / CHARACTER / "PROVENANCE.md").unlink()
        self.fails(root, "declared provenance file is missing")

    def test_digest_mismatch(self):
        root = self.copy()
        self.append(root / CHARACTER / "persona.md", "\n")
        self.fails(root, "sha256 mismatch for internal/oni/characters/oni/persona.md")

    def test_unrecorded_digest(self):
        root = self.copy()
        data = self.manifest(root)
        data["assets"][0]["sha256"] = None
        self.write_manifest(root, data)
        self.fails(root, "digest not recorded")

    def test_stale_declaration(self):
        root = self.copy()
        data = self.manifest(root)
        data["assets"].append(dict(data["assets"][-1], path=f"{CHARACTER}/wave.svg"))
        self.write_manifest(root, data)
        self.fails(root, "declared asset is missing")

    def test_noncommercial_license(self):
        root = self.copy()
        data = self.manifest(root)
        data["assets"][0]["spdx"] = "CC-BY-NC-4.0"
        self.write_manifest(root, data)
        self.fails(root, "undeclarable license")

    def test_unknown_declaration_key(self):
        root = self.copy()
        data = self.manifest(root)
        data["assets"][0]["permissions"] = ["hands.desktop"]
        self.write_manifest(root, data)
        self.fails(root, "entry needs exactly")

    def test_forbidden_marks_in_shipped_and_spec_text(self):
        targets = (f"{CHARACTER}/persona.md", "internal/oni/spec/notes.go",
                   "internal/oni/profile/testdata/broken/persona.md",
                   "internal/mcpserve/serve.go", "internal/dictate/insert.go",
                   "internal/reflex/launch.go", "internal/hands/browser.go",
                   "release/docs/oni-animation-spec.md")
        for mark in MARKS:
            for target in targets:
                with self.subTest(mark=mark, target=target):
                    root = self.copy()
                    self.append(root / target, f"\nSee {mark} here.\n")
                    shipped = target.startswith(native.ONI_CHARACTERS)
                    message = self.fails(root, f"in {target}", record=shipped)
                    self.assertIn("forbidden mark", message)
                    self.assertNotIn("sha256 mismatch", message)

    def test_lowercase_knot_is_not_a_mark(self):
        root = self.copy()
        self.append(root / "release/docs/oni-animation-spec.md", "\nA trefoil knot curve.\n")
        native.check_oni_assets(root)

    def test_notice_without_author(self):
        root = self.copy()
        notice = root / "NOTICE"
        notice.write_text(notice.read_text(encoding="utf-8").replace("Pranav Desu", "the author"),
                          encoding="utf-8")
        self.fails(root, "NOTICE lacks")

    def test_notice_without_apache_grant(self):
        root = self.copy()
        notice = root / "NOTICE"
        notice.write_text(notice.read_text(encoding="utf-8").replace("Apache-2.0", "MIT"),
                          encoding="utf-8")
        self.fails(root, "NOTICE lacks")

    def test_undeclared_model_and_voice_data(self):
        for name in ("internal/stt/base.onnx", "internal/llm/weights.gguf",
                     "vendor/example.com/m/tensor.bin", "internal/oni/voice/hello.wav",
                     "internal/voice/clip.OGG", "share/espeak-ng-data/phontab"):
            with self.subTest(name=name):
                root = self.copy()
                self.append(root / name, "data")
                self.fails(root, f"undeclared model or voice data: {name}")

    def test_declared_clip_pack_passes(self):
        root = self.copy()
        pack = "internal/oni/packs/demo"
        for name, text in (("hello.ogg", "audio"), ("LICENSE", "CC0-1.0\n"),
                           ("PROVENANCE.md", "Recorded for this test.\n")):
            self.append(root / pack / name, text)
        data = self.manifest(root)
        for name in ("hello.ogg", "LICENSE", "PROVENANCE.md"):
            data["assets"].append({"path": f"{pack}/{name}", "spdx": "CC0-1.0",
                                   "license_file": f"{pack}/LICENSE",
                                   "provenance": f"{pack}/PROVENANCE.md",
                                   "source": "Test recording", "sha256": None})
        self.write_manifest(root, data)
        native.record_oni_assets(root)
        native.check_oni_assets(root)

    def test_clip_pack_without_license_fails(self):
        root = self.copy()
        pack = "internal/oni/packs/demo"
        self.append(root / pack / "hello.ogg", "audio")
        self.append(root / pack / "PROVENANCE.md", "Recorded for this test.\n")
        data = self.manifest(root)
        for name in ("hello.ogg", "PROVENANCE.md"):
            data["assets"].append({"path": f"{pack}/{name}", "spdx": "CC0-1.0",
                                   "license_file": f"{pack}/LICENSE",
                                   "provenance": f"{pack}/PROVENANCE.md",
                                   "source": "Test recording", "sha256": None})
        self.write_manifest(root, data)
        self.fails(root, "declared license file is missing", record=True)


if __name__ == "__main__":
    unittest.main()
