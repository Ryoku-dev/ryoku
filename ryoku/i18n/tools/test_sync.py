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



class CatalogueExtractionTest(unittest.TestCase):
    def test_translation_bridge_call_is_catalogued(self):
        calls = sync.TR_CALL.findall('text: Translation.tr("Dock settings")\n'
                                     'accessibleName: Lithic.tr("Reset %1").arg(title)')
        self.assertEqual([body for _, body in calls], ["Dock settings", "Reset %1"])

    def test_catalogue_qml_and_dynamic_model_copy_are_catalogued(self):
        with tempfile.TemporaryDirectory() as store:
            qml = os.path.join(store, "barstyles", "demo", "qml", "panels")
            os.makedirs(qml)
            with open(os.path.join(qml, "Panel.qml"), "w", encoding="utf-8") as fh:
                fh.write('text: I18n.tr("Open panel")\n'
                         'property var rows: [{ label: "Pieces", '
                         'description: "Choose what this shell shows." }]\n')
            ignored = os.path.join(store, "profiles", "demo", "qml")
            os.makedirs(ignored)
            with open(os.path.join(ignored, "Ignored.qml"), "w", encoding="utf-8") as fh:
                fh.write('text: I18n.tr("Ignore profile QML")\n')

            keys = set()
            for root in sync._catalogue_qml_roots(store):
                sync._scan_qml(keys, root, include_model_fields=True)

        self.assertEqual(keys, {
            "Open panel", "Pieces", "Choose what this shell shows.",
        })

    def test_hub_registry_and_schema_copy_are_catalogued(self):
        with tempfile.TemporaryDirectory() as hub:
            os.makedirs(os.path.join(hub, "registry"))
            os.makedirs(os.path.join(hub, "schema"))
            os.makedirs(os.path.join(hub, "components", "mouse"))
            with open(os.path.join(hub, "components", "mouse", "buttons.js"), "w", encoding="utf-8") as fh:
                fh.write('function stepText(step, tr) { return tr("Hold %1 down").replace("%1", step.keys); }\n')
            with open(os.path.join(hub, "registry", "summaries.js"), "w", encoding="utf-8") as fh:
                fh.write('var still = {\n    "fonts-icons": "Fonts and the pointer",\n};\n'
                         'function corners(ctx) { return ctx.tr("%1 px corners").replace("%1", 4); }\n')
            with open(os.path.join(hub, "schema", "demo.js"), "w", encoding="utf-8") as fh:
                fh.write('var title = "Demo";\nvar lead = "What this page changes.";\n'
                         'var sections = [{ id: "glass", title: "Glass", missing: "Needs blur" }];\n'
                         'var rows = [{ id: "blur", section: "glass", label: "Blur", detail: "Frosted glass",\n'
                         '    keywords: ["frost"], control: "segmented", actionText: "Open", changeLabel: "Blur strength",\n'
                         '    options: [{ value: "strong", label: "Strong" }] },\n'
                         '    { id: "opts", section: "glass", label: "Options", control: "text", placeholder: "None" },\n'
                         '    { id: "dev", section: "glass", label: "Device", control: "text", placeholder: "/dev/video10" }];\n')

            keys = set()
            sync._scan_hub_data(keys, hub)

        self.assertEqual(keys, {
            "Fonts and the pointer", "%1 px corners", "Demo", "What this page changes.",
            "Glass", "Needs blur", "Blur", "Frosted glass", "Open", "Strong", "Hold %1 down",
            "Blur strength", "Options", "None", "Device",
        })

    def test_provider_rows_copy_is_catalogued(self):
        with tempfile.TemporaryDirectory() as wm:
            os.makedirs(os.path.join(wm, "demo"))
            with open(os.path.join(wm, "demo", "schema_anim.json"), "w", encoding="utf-8") as fh:
                json.dump([{"group": "WORKSPACE SWITCH", "key": "wm.demo.anim.mode",
                            "label": "Style", "desc": "How this animation runs",
                            "opts": ["default", "ease_out"],
                            "optLabels": {"default": "Demo default"}},
                           {"key": "", "label": "Key sounds", "ctl": "install",
                            "actionLabel": "Install sound engine"},
                           {"key": "wm.demo.anim.items[leaf=fade].enabled",
                            "label": "Fades (group) \u00b7 enabled", "desc": "All window fading"}], fh)

            keys = set()
            sync._scan_provider_rows(keys, wm)

        self.assertEqual(keys, {"Style", "How this animation runs", "Demo default", "Ease out",
                                "Key sounds", "Install sound engine",
                                "Fades (group) \u00b7 enabled", "All window fading", "Fades"})

    def test_store_environment_selects_catalogue_root(self):
        with tempfile.TemporaryDirectory() as store:
            qml = os.path.join(store, "subshells", "demo", "qml")
            os.makedirs(qml)
            with mock.patch.dict(os.environ, {"RYOKU_STORE_SRC": store}):
                self.assertEqual(sync._catalogue_qml_roots(), [qml])

    def test_store_argument_is_removed_from_command_arguments(self):
        root, args = sync._parse_store_arg(
            ["sync", "--engine", "llm", "--store", "../ryostore-revamped"])
        self.assertEqual(root, "../ryostore-revamped")
        self.assertEqual(args, ["sync", "--engine", "llm"])


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

    def test_plugin_descriptions_are_catalogued(self):
        roster = '{ID: "hyprglass", Name: "Glass", Desc: "Frosted glass windows.", Repo: "https://x"}'
        self.assertEqual(sync.PLUGIN_DESC.findall(roster), ["Frosted glass windows."])


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


class GlossaryTest(unittest.TestCase):
    # the short Settings titles a context-blind pass got wrong: a verb for a
    # page about monitors, side effects for software updates, and so on
    SETTINGS_TERMS = ("Displays", "Updates", "Windows", "Sounds", "Power", "Graphics",
                      "Accent", "Focus", "Look", "Recording", "Credits", "Keyring",
                      "Media", "Theme", "Wallpaper", "Shortcuts", "Home")

    def test_settings_titles_have_a_sense(self):
        for term in self.SETTINGS_TERMS:
            self.assertIn(term, sync.SENSE, term)
        self.assertIn("never the verb", sync.SENSE["Displays"])
        self.assertIn("side effects", sync.SENSE["Updates"])

    def test_prompt_carries_senses_and_the_heading_rule(self):
        prompt = sync.build_prompt("German", ["Displays", "Updates"])
        for term in self.SETTINGS_TERMS:
            self.assertIn(f'"{term}": {sync.SENSE[term]}', prompt)
        self.assertIn("noun phrase", prompt)
        for brand in ("Rashin", "RyoStore", "Ryogami"):
            self.assertIn(brand, sync.KEEP)


class OverridesTest(unittest.TestCase):
    def test_overrides_are_written_without_translating(self):
        lang = next(iter(sync.TARGETS))
        with tempfile.TemporaryDirectory() as trans:
            os.makedirs(os.path.join(trans, "overrides"))
            with open(os.path.join(trans, "en.json"), "w", encoding="utf-8") as fh:
                json.dump({"Displays": "Displays", "Keyboard": "Keyboard"}, fh)
            with open(os.path.join(trans, f"{lang}.json"), "w", encoding="utf-8") as fh:
                json.dump({"Displays": "Zeigt an", "Keyboard": "Tastatur"}, fh)
            with open(os.path.join(trans, "overrides", f"{lang}.json"), "w", encoding="utf-8") as fh:
                json.dump({"Displays": "Bildschirme", "Gone": "Weg"}, fh)

            with mock.patch.object(sync, "TRANS", trans), \
                    mock.patch.object(sync, "OVERRIDES", os.path.join(trans, "overrides")), \
                    mock.patch.object(sync, "google_translate_all", side_effect=AssertionError), \
                    mock.patch.object(sync, "llm_translate", side_effect=AssertionError), \
                    mock.patch("sys.stdout", io.StringIO()):
                self.assertEqual(sync.cmd_overrides([]), 0)
                with open(os.path.join(trans, f"{lang}.json"), encoding="utf-8") as fh:
                    got = json.load(fh)

        # the override wins, the rest of the catalog stays, and a string
        # English no longer has is not resurrected
        self.assertEqual(got, {"Displays": "Bildschirme", "Keyboard": "Tastatur"})

    def test_unknown_language_is_refused(self):
        with mock.patch("sys.stderr", io.StringIO()):
            self.assertEqual(sync.cmd_overrides(["xx"]), 2)


class ShippedOverridesTest(unittest.TestCase):
    """The repository's catalogs, which are what ships: every human fix is in
    them, keeps its placeholders, and the wrong senses stay gone."""

    def _overrides(self):
        for lang in sync.TARGETS:
            o = sync.load_json(os.path.join(sync.OVERRIDES, f"{lang}.json"))
            if o:
                yield lang, o

    def test_every_override_is_in_its_catalog(self):
        en = sync.load_json(os.path.join(sync.TRANS, "en.json"))
        for lang, overrides in self._overrides():
            catalog = sync.load_json(os.path.join(sync.TRANS, f"{lang}.json"))
            stale = sorted(k for k, v in overrides.items() if k in en and catalog.get(k) != v)
            self.assertEqual(stale, [], f"{lang}: run `sync.py overrides`")

    def test_overrides_keep_their_placeholders(self):
        for lang, overrides in self._overrides():
            for k, v in overrides.items():
                self.assertEqual(sync._ph(v), sync._ph(k), f"{lang}: {k!r}")

    def test_settings_titles_are_nouns(self):
        wrong = {("de", "Displays"): "Zeigt an", ("pl", "Displays"): "Wyświetla",
                 ("ar", "Displays"): "يعرض", ("bn", "Displays"): "প্রদর্শন করে",
                 ("nl", "Updates"): "Bijwerkingen", ("de", "Look"): "Schauen Sie",
                 ("de", "Focus"): "Konzentrieren Sie sich", ("vi", "Desktop"): "Shell"}
        for (lang, key), bad in wrong.items():
            catalog = sync.load_json(os.path.join(sync.TRANS, f"{lang}.json"))
            self.assertNotEqual(catalog.get(key), bad, f"{lang}: {key}")


if __name__ == "__main__":
    unittest.main()
