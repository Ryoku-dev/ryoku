#!/usr/bin/env python3

import io
import json
import os
import tempfile
import unittest
import urllib.error
from unittest import mock

import sync


class CopyDetectionTest(unittest.TestCase):
    def test_leading_placeholder_sentence_is_copy(self):
        self.assertTrue(sync._copy("%d orphaned package(s)"))
        self.assertTrue(sync._copy("%s is unavailable: %s"))

    def test_bare_placeholder_is_not_copy(self):
        self.assertFalse(sync._copy("%s"))
        self.assertFalse(sync._copy("%02d"))

    def test_placeholder_tokens_survive_translation_roundtrip(self):
        protected, tokens = sync.shield("%1m %2s left")
        self.assertEqual(protected, "__RYOKU_PH_0__m __RYOKU_PH_1__s left")
        self.assertEqual(sync.unshield("queda __RYOKU_PH_0__m __RYOKU_PH_1__s", tokens),
                         "queda %1m %2s")



class ShimaExtractionTest(unittest.TestCase):
    def test_translation_bridge_call_is_catalogued(self):
        calls = sync.TR_CALL.findall('text: Translation.tr("Dock settings")')
        self.assertEqual([body for _, body in calls], ["Dock settings"])

    def test_dynamic_model_copy_is_catalogued(self):
        fields = sync.QML_MODEL_FIELD.findall(
            '{ label: "Pieces", description: "Choose what Shima shows." }')
        self.assertEqual(fields, ["Pieces", "Choose what Shima shows."])


class ShortcutCatalogueExtractionTest(unittest.TestCase):
    def test_bind_row_copy_is_catalogued(self):
        row = '{ID: "shell.launcher", Category: "Shell", Label: "App launcher", Hint: "Open apps", Chord: "SUPER + Space"}'
        self.assertEqual(sync.BIND_FIELD.findall(row), ["Shell", "App launcher", "Open apps"])

    def test_compositor_reasons_are_catalogued(self):
        niri = '"window.pin": {reason: "niri has no pin-window action."},'
        self.assertEqual(sync.BIND_FIELD.findall(niri), ["niri has no pin-window action."])
        hypr = ('var hyprUnhonored = map[string]string{\n'
                '\t"column.first": "Hyprland has no column model.",\n}\n')
        blocks = sync.UNHONORED_MAP.findall(hypr)
        self.assertEqual([v for b in blocks for v in sync.MAP_VALUE.findall(b)],
                         ["Hyprland has no column model."])


class ProviderSchemaExtractionTest(unittest.TestCase):
    def test_provider_row_copy_is_catalogued(self):
        rows = [
            {"tab": "Look", "group": "OPACITY", "key": "wm.x.opaque", "label": "Keep it opaque",
             "desc": "Draw it solid", "ctl": "sw", "src": "desktop.json", "keywords": "glass"},
            {"tab": "Layout", "group": "SIZE", "key": "wm.x.mode", "label": "Size",
             "ctl": "seg", "opts": ["full", "custom"], "optLabels": {"full": "Full page"}},
            {"label": "Modifier", "opts": ["Super", "<super> <alt>"],
             "optLabels": {"<super> <alt>": "Super + Alt"}},
            {"tab": "Rules", "label": "Rules", "ctl": "list", "actionLabel": "Add rule",
             "fields": [{"name": "action", "label": "Action", "opts": ["float"]}]},
        ]
        self.assertEqual(sync.wm_schema_strings(rows), [
            "Look", "OPACITY", "Keep it opaque", "Draw it solid",
            "Layout", "SIZE", "Size", "Full page", "custom",
            "Modifier",
            "Rules", "Rules", "Add rule", "Action", "float"])

    def test_every_provider_schema_file_is_read(self):
        keys = sync.extract_keys()
        for name in ("hyprland/schema.json", "niri/schema_layout.json", "wayfire/schema.json"):
            path = os.path.join(sync.WM_ROOT, name)
            rows = json.load(open(path, encoding="utf-8"))
            label = next(r["label"] for r in rows if r.get("label"))
            self.assertIn(label, keys, name)


class LLMUnavailableTest(unittest.TestCase):
    def test_cmd_sync_returns_three_without_writing_catalog(self):
        lang = next(iter(sync.TARGETS))
        with tempfile.TemporaryDirectory() as trans:
            with open(os.path.join(trans, "en.json"), "w", encoding="utf-8") as fh:
                json.dump({"Power limit": "Power limit"}, fh)

            error = urllib.error.HTTPError(
                "https://example.invalid", 402, "Payment Required", {},
                io.BytesIO(b"insufficient credits"))
            stderr = io.StringIO()
            with mock.patch.object(sync, "TRANS", trans), \
                    mock.patch.object(sync, "OVERRIDES", os.path.join(trans, "overrides")), \
                    mock.patch.object(sync, "load_llm_cfg", return_value={"key": "test"}), \
                    mock.patch.object(sync.urllib.request, "urlopen", side_effect=error), \
                    mock.patch("sys.stderr", stderr):
                result = sync.cmd_sync(["--engine", "llm", lang])

            self.assertEqual(result, 3)
            self.assertFalse(os.path.exists(os.path.join(trans, f"{lang}.json")))
            self.assertIn("translation provider unavailable: HTTP 402", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
