import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const Apps = require("./apps.js");
const JobEvents = require("../../../../ui/lib/jobevents.js");

const eq = (got, want, message) => assert.deepEqual(got, want, message);

// Ryoku apps shelf
const listed = Apps.parseRyokuApps(JSON.stringify({ schema: 1, apps: [
    { id: "ryotunes", name: "Ryotunes", summary: "Music", installed: true, version: "1.9.0", latest: "2.0.0",
      update: true, size_bytes: 42000000, removable: true, builtin: false, desktop_id: "ryotunes.desktop",
      lane: "apprelease", channel: "prerelease" },
    { id: "ryoport", name: "Ryoport", installed: false, latest: "1.2.0", update: true, removable: true, lane: "apprelease" },
    { id: "ryomanager", name: "Ryomanager", installed: true, version: "0.4.0", update: false, removable: false,
      desktop_id: "ryomanager.desktop", lane: "repo" },
    { id: "ryostore", name: "Ryostore", installed: true, builtin: true, removable: false }
] }));
eq(listed.map(a => a.id), ["ryotunes", "ryoport", "ryomanager"], "the built-in store stays off the shelf");
eq(listed[0].channel, "prerelease", "channel kept");
eq(listed[1].update, false, "an app that is not installed has nothing to update");
eq(Apps.ryokuActions(listed[0]), ["update", "open", "remove"], "installed with an update");
eq(Apps.ryokuActions(listed[1]), ["install"], "not installed");
eq(Apps.ryokuActions(listed[2]), ["open"], "protected app is never removable here");
eq(Apps.ryokuCommand(listed[0], "update"), ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "ryoku", "update", "ryotunes"], "host verb, cancellable through stdin");
// the commands Ryostore builds must be the ones the shared job knows how to stop
eq(JobEvents.cancelsByStdin(Apps.ryokuCommand(listed[0], "update")), true, "a root job is stopped through its stdin");
eq(JobEvents.cancellable(Apps.packageCommand({ kind: "system", source: "pacman", installName: "a" }, "install")), true, "a system package job can be stopped");
eq(JobEvents.cancellable(Apps.packageCommand({ kind: "flatpak", source: "flatpak", installName: "a" }, "install")), true, "a Flatpak job can be stopped");
eq(Apps.displayVersion("1:2.0.0-1"), "2.0.0-1", "the epoch is not shown");
eq(Apps.displayVersion("0.3.0"), "0.3.0", "plain versions stay");
eq(Apps.gpkVersion("gpk v0.6.8\n"), [0, 6, 8], "version from --version");
eq(Apps.gpkVersion("v0.7.0-dev"), [0, 7, 0], "a pre-release reads as its release");
eq(Apps.gpkCanInstall("gpk v0.6.8"), false, "0.6 cannot report progress");
eq(Apps.gpkCanInstall("v0.6.10"), false, "0.6.10 is still older than 0.7.0");
eq(Apps.gpkCanInstall("v0.7.0-dev"), true, "0.7.0 builds install");
eq(Apps.gpkCanInstall("v1.0.0"), true, "newer majors install");
eq(Apps.gpkCanInstall(""), true, "an unknown gpk is let through");
eq(Apps.searchVersion('{"gpk_version":"v0.6.8","schema":1,"data":[]}'), "v0.6.8", "version from the envelope");
assert.throws(() => Apps.parseRyokuApps("{}"), /apps are missing/);

// gpk search
const rows = Apps.parseSearch(JSON.stringify({ gpk_version: "v0.7.0", schema: 1, data: [
    { name: "Firefox", id: "org.mozilla.firefox", source: "flatpak", version: "133.0", description: "Web browser" },
    { name: "firefox", source: "pacman", version: "133.0-1", description: "Standalone web browser", installed: true, size_bytes: 250000000 },
    { name: "firefox-nightly", source: "aur", version: "135.0a1" },
    { name: "requests", source: "pip", version: "2.0" },
    { name: "firefox", source: "pacman", version: "133.0-1" },
    { name: "", source: "pacman" }
] }));
eq(rows.map(r => r.key), ["flatpak:org.mozilla.firefox", "pacman:firefox", "aur:firefox-nightly"], "kept sources, no duplicates");
eq(rows.map(r => r.kind), ["flatpak", "system", "aur"], "row kinds");
eq(rows[0].name, "Firefox", "display name kept");
eq(rows[1].installed, true, "installed state");
eq(Apps.sourceChip("flatpak"), "FLATHUB", "flatpak chip names Flathub");
eq(Apps.sourceChip("pacman"), "PACMAN", "system chip names the manager");
eq(Apps.packageCommand(rows[0], "install"), ["gpk", "install", "--manager", "flatpak", "--yes", "--events", "org.mozilla.firefox"], "flatpak runs as the user");
eq(Apps.packageCommand(rows[1], "remove"), ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "remove", "--manager", "pacman", "firefox"], "system packages go through the host helper");
eq(Apps.packageCommand(rows[2], "install"), null, "the AUR never runs in place");
eq(Apps.aurLine(rows[2], "install"), "gpk install --manager aur 'firefox-nightly'", "AUR terminal line");
eq(Apps.removePlanCommand(rows[1]), ["gpk", "remove", "--manager", "pacman", "--json", "firefox"], "removal plan is read first, as the user");
eq(Apps.removePlanCommand(rows[2]), null, "no plan for the AUR here");
eq(Apps.requiredBy(JSON.stringify({ gpk_version: "v0.7.0", schema: 1, data: { action: "remove", steps: [
    { manager: "pacman", name: "gcr", required_by: ["gnome-keyring", "seahorse"] },
    { manager: "pacman", name: "gcr-4", required_by: ["gnome-keyring"] },
    { manager: "pacman", name: "lone" }
] } })), ["gnome-keyring", "seahorse"], "required_by gathered once each");
eq(Apps.requiredBy(JSON.stringify({ data: { steps: [] } })), [], "nothing needs it");
eq(Apps.shellQuote("it's"), "'it'\\''s'", "shell quoting");
eq(Apps.searchCommand("fire fox").slice(-3), ["--manager", "flatpak,pacman,aur,dnf,xbps", "fire fox"], "search command");
assert.throws(() => Apps.parseSearch("{}"), /search data is missing/);

// sources
const sources = Apps.parseSources(JSON.stringify({ schema: 1, sources: [
    { id: "ryostore", name: "Ryostore", url: "https://a", official: true, enabled: true, status: "ok", items: 300 },
    { id: "ryostore-revamped", name: "Ryostore shells", url: "https://b", official: true, enabled: true, status: "ok" },
    { id: "mine", name: "Mine", url: "https://c", official: false, enabled: false, status: "disabled" },
    { id: "gone", name: "Gone", url: "https://d", status: "removed", installed: 2, note: "kept" }
] }));
eq(sources.official.map(s => s.id), ["ryostore", "ryostore-revamped"], "official list");
eq(sources.mine.map(s => [s.id, s.enabled, s.status]), [["mine", false, "disabled"], ["gone", true, "removed"]], "your sources");
eq(Apps.sourceUrlProblem(""), "empty", "empty url");
eq(Apps.sourceUrlProblem("https://store.example.org/ryostore"), "", "https");
eq(Apps.sourceUrlProblem("http://example.org"), "scheme", "plain http refused");
eq(Apps.sourceUrlProblem("http://127.0.0.1:8000/"), "", "http on this computer");
eq(Apps.sourceUrlProblem("file:///srv/store"), "", "local tree");
eq(Apps.formatBytes(0), "", "unknown size");
eq(Apps.formatBytes(42000000), "40.1 MB", "megabytes");
eq(Apps.formatBytes(250000000), "238 MB", "large megabytes");
eq(Apps.formatBytes(3221225472), "3.0 GB", "gigabytes");

console.log("RYOSTORE-APPS-HELPERS-PASS");
