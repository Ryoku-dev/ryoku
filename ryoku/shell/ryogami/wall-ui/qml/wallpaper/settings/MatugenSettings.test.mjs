import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const source = readFileSync(new URL("./MatugenSettings.qml", import.meta.url), "utf8");
const panel = readFileSync(new URL("../SettingsPanel.qml", import.meta.url), "utf8");
const moduleTypes = readFileSync(new URL("./qmldir", import.meta.url), "utf8");

assert.match(source, /Column\s*{/,
    "Matugen must keep the known-working settings page root");
assert.doesNotMatch(panel, /id:\s*matugenViewport/,
    "Matugen subpages must not be buried in a scrolling parent");
assert.match(source, /property string activePage:\s*"matugen"/,
    "Matugen must open on its original controls");
assert.match(source, /label:\s*I18n\.tr\("MATUGEN"\)[\s\S]*label:\s*I18n\.tr\("PALETTE BRIDGE"\)/,
    "Matugen must expose a second row of subpages");
assert.match(panel, /activeTab === "performance" \|\| settingsPanel\.activeTab === "matugen"/,
    "Matugen must use the wide settings layout");

const external = source.indexOf('title: I18n.tr("External Matugen")');
const bridge = source.indexOf("PaletteBridgeSettings {");
assert.ok(external >= 0 && bridge > external,
    "existing Matugen controls must remain visible before Palette Bridge controls");
assert.match(moduleTypes, /^PaletteBridgeSettings 1\.0 PaletteBridgeSettings\.qml$/m,
    "PaletteBridgeSettings must be registered so Matugen can instantiate it");

console.log("PASS: Matugen and Palette Bridge render as tall sibling subpages");
