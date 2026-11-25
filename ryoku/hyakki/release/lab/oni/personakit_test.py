# SPDX-License-Identifier: GPL-3.0-only
"""Consumer tests for final evidence admission, never synthetic personas.
Run explicitly after source freeze:
  PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s release/lab/oni -p personakit_test.py
"""
import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import personakit as kit


class EvidenceAdmission(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="hyakki-persona-tests-", dir="/dev/shm")
        self.root = Path(self.tmp.name)
        self.addCleanup(self.tmp.cleanup)

    def test_receipt_rejects_empty_or_failed_steps_despite_green_summary(self):
        path = self.root / "receipt.json"
        for value in ({"pass": True}, {"pass": True, "steps": []},
                      {"pass": True, "steps": [{"name": "task", "pass": False}]},
                      {"pass": True, "steps": [{"name": "task"}]}):
            with self.subTest(value=value):
                kit.dump(path, value)
                with self.assertRaises(RuntimeError):
                    kit.receipt(path)

    def test_nonempty_consumer_receipt_is_accepted(self):
        path = self.root / "receipt.json"
        value = {"pass": True, "steps": [{"name": "rendered document saved", "pass": True}]}
        kit.dump(path, value)
        self.assertEqual(kit.receipt(path), value)

    def test_existing_evidence_is_not_replaced(self):
        out = kit.private_out(self.root / "run")
        payload = out / "capture.png"
        payload.write_bytes(b"retain prior run")
        with self.assertRaises(RuntimeError):
            kit.private_out(out)
        self.assertEqual(payload.read_bytes(), b"retain prior run")

    def test_output_cannot_escape_tmpfs_through_symlink(self):
        (self.root / "outside").symlink_to("/tmp", target_is_directory=True)
        with self.assertRaises(RuntimeError):
            kit.private_out(self.root / "outside" / "new-evidence")

    def test_artifacts_refuse_symlinked_recording(self):
        (self.root / "audio.wav").symlink_to("/etc/hostname")
        with self.assertRaises(RuntimeError):
            kit.artifacts(self.root)

    def test_snapshot_change_invalidates_finished_work(self):
        frozen = {"source_sha256": "before", "binaries": {"oni": "same"}}
        for changed in ({"source_sha256": "after", "binaries": {"oni": "same"}},
                        {"source_sha256": "before", "binaries": {"oni": "changed"}}):
            with self.subTest(changed=changed), patch.object(kit, "identity", return_value=changed):
                with self.assertRaises(RuntimeError):
                    kit.stable(frozen)

    def cell(self, root, wm):
        where = kit.private_out(self.root / f"{root}-{wm}")
        (where / "observed.txt").write_text("immutable capture payload\n")
        snapshot = {"source_sha256": "source-a", "binary_sha256": "bins-a"}
        report = {"pass": True, "execution_root": root, "wm": wm, "snapshot": snapshot,
                  "steps": [{"name": f"phase{x}", "pass": True} for x in kit.PHASES]
                           + [{"name": x, "pass": True} for x in kit.PERSONAS],
                  "limits": [], "artifacts_sha256": kit.artifacts(where)}
        kit.dump(where / "phase18.json", report)
        return where

    def index(self, cells, name="index"):
        with patch.object(kit, "identity", return_value={"source_sha256": "source-a", "binary_sha256": "bins-a"}), contextlib.redirect_stdout(io.StringIO()):
            rc = kit.matrix(self.root / name, cells)
        return rc, json.loads((self.root / name / "matrix.json").read_text())

    def four_cells(self):
        return [self.cell(r, w) for r in ("checkout", "package") for w in ("niri", "hyprland")]

    def test_matrix_requires_all_four_distinct_cells(self):
        cells = self.four_cells()
        rc, report = self.index(cells[:3])
        self.assertNotEqual(rc, 0)
        self.assertFalse(report["pass"])
        rc, report = self.index(cells[:3] + [cells[0]], "duplicates")
        self.assertNotEqual(rc, 0)
        self.assertFalse(report["pass"])

    def test_matrix_rejects_changed_audio_or_capture(self):
        cells = self.four_cells()
        (cells[1] / "observed.txt").write_text("changed after acceptance\n")
        rc, report = self.index(cells)
        self.assertNotEqual(rc, 0)
        self.assertIn("artifact missing/changed", report["error"])

    def test_matrix_rejects_narrowed_suite(self):
        cells = self.four_cells()
        path = cells[0] / "phase18.json"
        report = json.loads(path.read_text())
        report["steps"].pop(0)
        kit.dump(path, report)
        rc, result = self.index(cells)
        self.assertNotEqual(rc, 0)
        self.assertIn("narrowed", result["error"])

    def test_four_matching_cells_can_be_indexed_without_claiming_live_desktop(self):
        rc, result = self.index(self.four_cells())
        self.assertEqual(rc, 0)
        self.assertTrue(result["pass"])
        self.assertIn("NOT live desktop", result["scope"])


if __name__ == "__main__":
    unittest.main()
