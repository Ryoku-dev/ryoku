import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

// load apps.js the way the QML engine links it; the progress events are Ryoku.Ui's
// (ryoku/ui/lib/jobevents.test.mjs)
const here = path.dirname(fileURLToPath(import.meta.url));
function load(file) {
    const c = vm.createContext({});
    vm.runInContext(fs.readFileSync(path.join(here, file), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), c);
    return c;
}
const A = load("apps.js");
const plain = v => JSON.parse(JSON.stringify(v));

const MB = 1024 * 1024;
const row = o => Object.assign({ no_display: false, terminal: false, location: "system", source: "pacman" }, o);
const gpk = rows => JSON.stringify({ gpk_version: "0.7.0", schema: 1, data: rows });

const sample = [
    row({ desktop_id: "firefox", name: "Firefox", generic_name: "Web browser", package: "firefox", size_bytes: 250 * MB }),
    row({ desktop_id: "steam.desktop", name: "Steam", package: "steam", size_bytes: 5 * 1024 * MB }),
    row({ desktop_id: "kitty", name: "kitty", package: "kitty", size_bytes: 29 * MB }),
    row({ desktop_id: "kitty-open", name: "kitty URL Launcher", package: "kitty", size_bytes: 29 * MB, no_display: true }),
    row({ desktop_id: "htop", name: "Htop", package: "htop", size_bytes: 400 * 1024 }),
    row({ desktop_id: "code", name: "Visual Studio Code", package: "visual-studio-code-bin", source: "aur", size_bytes: 400 * MB }),
    row({ desktop_id: "org.libreoffice.LibreOffice.writer", name: "LibreOffice Writer", package: "org.libreoffice.LibreOffice",
          source: "flatpak", location: "flatpak", size_bytes: 1200 * MB }),
    row({ desktop_id: "org.libreoffice.LibreOffice.calc", name: "LibreOffice Calc", package: "org.libreoffice.LibreOffice",
          source: "flatpak", location: "flatpak", size_bytes: 1200 * MB }),
    row({ desktop_id: "obsidian", name: "Obsidian", package: "Obsidian", source: "local", location: "user", size_bytes: 200 * MB }),
    row({ desktop_id: "webapp", name: "A web app", source: "local", location: "user" }),
    row({ desktop_id: "ryotunes", name: "Ryotunes", package: "ryotunes", size_bytes: 46 * MB })
];
const apps = A.parseApps(gpk(sample));

test("apps read from gpk: one per shown desktop entry, ids without .desktop", () => {
    assert.equal(apps.length, 10);
    assert.ok(!apps.some(a => a.id === "kitty-open"), "a hidden entry never shows");
    assert.equal(apps.find(a => a.name === "Steam").id, "steam");
    assert.equal(apps.find(a => a.id === "firefox").summary, "Web browser");
    assert.equal(apps.find(a => a.id === "webapp").bytes, 0);
    assert.equal(A.parseApps(JSON.stringify({ data: { apps: [sample[0]] } })).length, 1, "data.apps is read too");
    assert.throws(() => A.parseApps("not json"));
    assert.throws(() => A.parseApps(JSON.stringify({ data: { nope: 1 } })));
});

test("each app falls in one size group by its package's installed size", () => {
    assert.equal(A.groupOf(5 * 1024 * MB), "over-1gb");
    assert.equal(A.groupOf(1024 * MB), "over-1gb");
    assert.equal(A.groupOf(1024 * MB - 1), "100mb-1gb");
    assert.equal(A.groupOf(100 * MB), "100mb-1gb");
    assert.equal(A.groupOf(10 * MB), "10-100mb");
    assert.equal(A.groupOf(10 * MB - 1), "under-10mb");
    assert.equal(A.groupOf(0), "unknown");
    assert.equal(A.groupOf(undefined), "unknown");
    assert.deepEqual(plain(A.grouped(apps, "size").map(g => g.id)), ["over-1gb", "100mb-1gb", "10-100mb", "under-10mb", "unknown"]);
    assert.deepEqual(plain(A.grouped(apps, "name")[0].apps.map(a => a.name)), ["LibreOffice Calc", "LibreOffice Writer", "Steam"]);
});

test("sizes read as people write them", () => {
    assert.equal(A.humanSize(5 * 1024 * MB), "5.0 GB");
    assert.equal(A.humanSize(312 * MB), "312 MB");
    assert.equal(A.humanSize(4.2 * MB), "4.2 MB");
    assert.equal(A.humanSize(400 * 1024), "400 KB");
    assert.equal(A.humanSize(10), "1 KB");
    assert.equal(A.humanSize(2 * 1024 * MB, s => s.replace("GB", "Go")), "2.0 Go");
});

test("sorting by size puts the largest first and unknown sizes last; by name A to Z", () => {
    const bySize = A.sorted(apps, "size").map(a => a.name);
    assert.equal(bySize[0], "Steam");
    assert.equal(bySize[bySize.length - 1], "A web app");
    assert.deepEqual(plain(A.sorted(apps, "name").slice(0, 3).map(a => a.name)), ["A web app", "Firefox", "Htop"]);
});

test("filters by where an app came from, and by words", () => {
    assert.deepEqual(plain(A.filtered(apps, "aur", "").map(a => a.id)), ["code"]);
    assert.deepEqual(plain(A.filtered(apps, "flatpak", "").map(a => a.id)),
        ["org.libreoffice.LibreOffice.writer", "org.libreoffice.LibreOffice.calc"]);
    assert.deepEqual(plain(A.filtered(apps, "account", "").map(a => a.id)), ["obsidian", "webapp"]);
    assert.equal(A.filtered(apps, "system", "").length, 5);
    assert.deepEqual(plain(A.filtered(apps, "all", "browser").map(a => a.id)), ["firefox"]);
    assert.deepEqual(plain(A.kindCounts(apps)), { system: 5, aur: 1, flatpak: 2, account: 2 });
});

test("space counts a package once, however many apps it opens", () => {
    const s = A.space(apps);
    assert.equal(s.apps, 10);
    assert.equal(s.packages, 9);
    assert.equal(s.unknown, 1);
    assert.equal(s.total, (250 + 5 * 1024 + 29 + 400 + 1200 + 200 + 46) * MB + 400 * 1024);
    assert.equal(s.bySource.find(x => x.id === "flatpak").bytes, 1200 * MB);
    assert.equal(s.bySource.find(x => x.id === "flatpak").count, 1);
    assert.deepEqual(plain(s.byGroup.map(g => [g.id, g.count])),
        [["over-1gb", 2], ["100mb-1gb", 3], ["10-100mb", 2], ["under-10mb", 1], ["unknown", 1]]);
    assert.equal(A.space([]).total, 0);
    assert.equal(A.space([]).bySource.length, 0);
});

test("a removal runs where it may: as root through ryoku-host, as you, or in a terminal", () => {
    const find = id => apps.find(a => a.id === id);
    assert.deepEqual(plain(A.removeCommand(find("firefox"))),
        { kind: "run", argv: ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "remove", "--manager", "pacman", "firefox"] });
    assert.deepEqual(plain(A.removeCommand(find("org.libreoffice.LibreOffice.calc"))),
        { kind: "run", argv: ["gpk", "remove", "-m", "flatpak", "--yes", "--events", "org.libreoffice.LibreOffice"] });
    assert.deepEqual(plain(A.removeCommand(find("obsidian"))),
        { kind: "run", argv: ["gpk", "remove", "-m", "local", "--yes", "--events", "Obsidian"] });
    assert.deepEqual(plain(A.removeCommand(find("code"))), { kind: "terminal", line: "gpk remove -m aur 'visual-studio-code-bin'" });
    assert.equal(A.removeCommand(find("webapp")), null, "nothing owns it, so nothing removes it");
    assert.equal(A.removeCommand(Object.assign({}, find("firefox"), { source: "snap" })), null);
    assert.deepEqual(plain(A.ryokuCommand("update", "ryotunes")),
        ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "ryoku", "update", "ryotunes"]);
});

test("the removal plan names the packages that need it", () => {
    const plan = JSON.stringify({ gpk_version: "v0.7.0", schema: 1, data: { action: "remove", steps: [
        { manager: "pacman", name: "gcr", required_by: ["gnome-keyring"] },
        { manager: "pacman", name: "x", required_by: ["gnome-keyring", "seahorse"] }] } });
    assert.deepEqual(plain(A.requiredBy(plan)), ["gnome-keyring", "seahorse"]);
    assert.deepEqual(plain(A.requiredBy('{"data":{"steps":[{"name":"kitty"}]}}')), []);
    assert.deepEqual(plain(A.requiredBy("not json")), []);
    assert.deepEqual(plain(A.planCommand(apps.find(a => a.id === "firefox"))), ["gpk", "remove", "-m", "pacman", "--json", "firefox"]);
    assert.equal(A.planCommand(apps.find(a => a.id === "webapp")), null);
});

test("a removal names the other apps of its package and the space it frees", () => {
    const calc = apps.find(a => a.id === "org.libreoffice.LibreOffice.calc");
    assert.deepEqual(plain(A.removal(calc, apps)),
        { names: ["LibreOffice Writer", "LibreOffice Calc"], others: ["LibreOffice Writer"], bytes: 1200 * MB });
    const web = apps.find(a => a.id === "webapp");
    assert.deepEqual(plain(A.removal(web, apps).others), []);
});

test("an entry in your own folders is still its package manager's; Ryoku's own entries are never removed here", () => {
    const more = A.parseApps(gpk([
        row({ desktop_id: "nvim", name: "Neovim", package: "neovim", location: "user", size_bytes: 30 * MB }),
        row({ desktop_id: "ryoku-hub", name: "Ryoku Settings", package: "ryoku-desktop", location: "user", size_bytes: 125 * MB }),
        row({ desktop_id: "zen", name: "Zen Browser", package: "Zen Browser", source: "local", location: "user", size_bytes: 440 * MB }),
        row({ desktop_id: "vendor", name: "Vendor tool", source: "local", location: "system" })
    ]));
    assert.deepEqual(plain(more.map(a => A.kindOf(a))), ["system", "system", "account", "system"]);
    assert.equal(A.partOfRyoku(more[1]), true);
    assert.equal(A.removeCommand(more[1]), null);
    assert.deepEqual(plain(A.removeCommand(more[0]).argv.slice(-3)), ["--manager", "pacman", "neovim"]);
    assert.equal(A.removeCommand(more[3]), null);
});

test("Ryoku's own apps are read apart and left out of the list", () => {
    const ryoku = A.parseRyoku(JSON.stringify({ schema: 1, apps: [
        { id: "ryotunes", name: "Ryotunes", installed: true, version: "1.9.4", latest: "2.0.0", update: true,
          size_bytes: 46 * MB, removable: true, builtin: false, desktop_id: "ryotunes.desktop", channel: "prerelease" },
        { id: "ryostore", name: "Ryostore", installed: true, removable: true, builtin: true, desktop_id: "ryostore" },
        { name: "no id" }
    ] }));
    assert.equal(ryoku.length, 2);
    assert.equal(ryoku[0].desktopId, "ryotunes");
    assert.equal(ryoku[0].update, true);
    assert.equal(ryoku[1].removable, false, "a built-in app is never removable");
    assert.ok(!A.withoutRyoku(apps, ryoku).some(a => a.id === "ryotunes"));
    assert.equal(A.withoutRyoku(apps, ryoku).length, 9);
    assert.throws(() => A.parseRyoku("{}"));
});

test("showing or hiding an app edits the launcher's hidden list once", () => {
    assert.deepEqual(plain(A.withHidden(["a", "b"], "c", true)), ["a", "b", "c"]);
    assert.deepEqual(plain(A.withHidden(["a", "c"], "c", true)), ["a", "c"]);
    assert.deepEqual(plain(A.withHidden(["a", "c", "c"], "c", false)), ["a"]);
    assert.deepEqual(plain(A.withHidden(undefined, "c", true)), ["c"]);
});

test("a gpk older than 0.7.0 is told apart, and its search output never reads as an empty list", () => {
    assert.equal(A.gpkListsApps("gpk v0.6.8\n"), false);
    assert.equal(A.gpkListsApps("gpk 0.6.10-1"), false);
    assert.equal(A.gpkListsApps("gpk v0.7.0"), true);
    assert.equal(A.gpkListsApps("gpk v0.7.0-dev"), true);
    assert.equal(A.gpkListsApps("gpk 1.0.0"), true);
    assert.equal(A.gpkListsApps("gpk dev"), true, "a development build is asked");
    assert.equal(A.gpkVersion("gpk v0.6.8"), "0.6.8");
    // gpk 0.6 takes `apps` for a search and prints packages
    assert.throws(() => A.parseApps(JSON.stringify({ gpk_version: "v0.6.8", schema: 1,
        data: [{ name: "apps", version: "0.2.2", source: "cargo" }] })), /old gpk/);
    assert.equal(A.parseApps(JSON.stringify({ data: [] })).length, 0, "no apps at all is still a list");
});
