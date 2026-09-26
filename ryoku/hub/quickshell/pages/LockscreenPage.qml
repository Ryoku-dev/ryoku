pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Shapes
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Lockscreen management. RyoStore owns discovery and installation; this page
// lists installed qylock themes and keeps activation, live preview, and greeter
// application in Settings.
// page owns the whole content region and draws its own head, states and grid.
// The pick only swaps the look, never the login. Every value is a Token; the
// live skin thumbnail is the one permitted specimen of colour (a lock preview),
// everything else is paper and ink.
Item {
    id: pg

    property var hub
    // A full-bleed page draws the whole content region itself: the shell hides
    // its side panel and global action bar and keeps only the rail.
    readonly property bool fullBleed: true

    // ── state (backend unchanged from the old page) ─────────────────────────
    property var skins: []
    property string active: ""
    property bool loading: true
    property bool loadFailed: false
    property string pendingSlug: ""   // "" idle, else the slug being applied
    property string error: ""
    // "none installed" is a valid empty result; loadFailed is a read failure.
    // Kept apart so the page shows a Store nudge for the first and a retry for
    // the second, never one message that blames the user for a backend that
    // couldn't answer.
    readonly property bool emptyInstalled: !pg.loading && !pg.loadFailed && pg.skins.length === 0

    // The stable launcher enters the generation lease before selecting the
    // replaceable lock client, then forwards the named preview skin.
    readonly property string lockLauncher: "ryoku-qylock-lock"
    // the rail's search box drives this; skins filter live against it.
    readonly property string query: (pg.hub && pg.hub.query) ? ("" + pg.hub.query) : ""

    // responsive column count, same breakpoints as the old bento grid.
    readonly property int cols: width >= 1320 ? 4 : (width >= 980 ? 3 : (width >= 640 ? 2 : 1))

    // vector glyph paths (viewBox 24), inlined so the page carries no icon
    // dependency of its own.
    readonly property string pLock: "M5 11h14a1 1 0 0 1 1 1v8a1 1 0 0 1 -1 1H5a1 1 0 0 1 -1 -1v-8a1 1 0 0 1 1 -1z M8 11V7.5a4 4 0 0 1 8 0V11 M12 15v2.5"
    readonly property string pPlay: "M8 5.4l11 6.6 -11 6.6z"
    readonly property string pChevron: "M6 9.5l6 6 6 -6"
    readonly property string pRefresh: "M21 12a9 9 0 1 1 -2.6 -6.4 M21 3v5h-5"
    readonly property string pFingerprint: "M2 12a10 10 0 0 1 18-6 M21.8 16c.2-2 .13-5.35 0-6 M5 19.5C5.5 18 6 15 6 12a6 6 0 0 1 .34-2 M9 6.8a6 6 0 0 1 9 5.2v2 M12 10a2 2 0 0 0 -2 2c0 1.02-.1 2.51-.26 4 M14 13.12c0 2.38 0 6.38-1 8.88 M17.29 21.02c.12-.6.43-2.3.5-3.02 M8.65 22c.21-.66.45-1.32.57-2 M2 16h.01"
    readonly property string pKey: "M14 7a4 4 0 1 0 0 8a4 4 0 0 0 3.58-2.21H22v-2h-2v-2h-2.42A4 4 0 0 0 14 7"
    readonly property string pCheck: "M4.5 12.5l5 5L19.5 7"
    readonly property string pX: "M6 6l12 12 M18 6L6 18"

    // ── sign-in keyring state (`ryoku keyring status --json`) ───────────────
    // A separate concern from the skin gallery: how the GNOME keyring unlocks at
    // sign-in. Read live so the section reflects PAM/keyring reality, applied
    // through `ryoku keyring set` (which pops polkit for the root PAM half, the
    // same UX as applying a skin reskins the greeter).
    property string kmode: ""
    property bool kdaemon: false
    property string kdefName: ""
    property string kdefFormat: ""      // encrypted · plaintext · absent
    property var knotes: []
    property bool kloading: true
    property string kerror: ""
    property string kpending: ""        // mode being applied, "" idle
    property string kconvertFor: ""     // "" hidden, else the mode awaiting a password
    property bool kconfirmReset: false
    property string kstdin: ""          // password piped to the next set, never argv
    // never-ask needs a blank keyring; an encrypted one blocks the switch until
    // the user converts it or starts fresh.
    readonly property bool kNeverAskBlocked: pg.kdefFormat === "encrypted"

    readonly property string kStatusLine: {
        if (pg.kloading)
            return I18n.tr("Checking\u2026");
        var parts = [];
        if (pg.kdefFormat === "encrypted")
            parts.push(I18n.tr("your keyring is password-protected"));
        else if (pg.kdefFormat === "plaintext")
            parts.push(I18n.tr("your keyring is unlocked, no password"));
        else if (pg.kdefFormat === "absent")
            parts.push(I18n.tr("no keyring created yet"));
        parts.push(pg.kdaemon ? I18n.tr("keyring agent running") : I18n.tr("keyring agent not running"));
        return parts.join("  \u00b7  ");
    }

    // ── fingerprint unlock state (fprintd, live; no root writes) ────────────
    property bool ffpEnabled: true            // ~/.config/qylock/fingerprint toggle
    property bool fdaemon: false              // fprintd service reachable
    property bool fready: false               // a device is present
    property string fdevice: ""               // D-Bus object path
    property string fdeviceName: ""           // human sensor name, e.g. "Synaptics Sensors"
    property var ffingers: []                 // enrolled names from fprintd
    property var fnames: ({})                 // fingerprint name -> user label map
    property bool floading: true
    property string ferr: ""
    property string fpending: ""              // "" | "enroll" | "verify" | "del"
    property bool foverlay: false             // the enroll/verify modal
    property string foverlayMode: ""          // "enroll" | "verify"
    property string fstatus: ""               // latest raw fprintd status token
    property string fresult: ""               // last finished action result
    property string fterm: ""                 // accumulated terminal output (log strip)
    property var foffsets: ({})               // consumed length per stream, so live
                                              // parsing never counts a line twice
    property int fstagesPassed: 0             // enroll-stage-passed count
    property int fstagesTotal: 0              // num-enroll-stages; 0 = unknown
    property string fuser: Quickshell.env("USER") || "traveler"
    // naming flow after successful enroll
    property string fnewFinger: ""            // finger name just enrolled (from fprintd)
    property string fnameDraft: ""            // user's draft name for the new finger
    property bool fnaming: false              // showing the name field after enroll
    // two-step destructive confirms; a stray click can never silently erase a
    // print, and the armed state falls back on its own after 3s.
    property bool frmConfirm: false           // REMOVE ALL armed
    property string fdelConfirmFor: ""        // per-finger delete armed
    property string fdelTarget: ""            // finger being deleted right now

    // grosshack line in /etc/pam.d/{sudo,sddm,polkit-1}; pkexec applies/removes it.
    property bool fpamModuleOk: false         // pam_fprintd_grosshack.so present
    property bool fsudoOn: false              // grosshack line in /etc/pam.d/sudo
    property bool fsddmOn: false              // grosshack line in /etc/pam.d/sddm
    property bool fpamLoading: true
    property bool fsudoPending: false         // an apply/remove is in flight
    property bool fsddmPending: false
    property bool fpolkitOn: false            // grosshack line in /etc/pam.d/polkit-1
    property bool fpolkitPending: false
    readonly property string fpamLine: "auth        sufficient    pam_fprintd_grosshack.so"

    function fpamReload() {
        pg.fpamLoading = true;
        pamStatusProc.running = true;
    }
    function fpamToggle(target, on) {         // target: "sudo" | "sddm"
        if (pg.fpending !== "" || pg.fsudoPending || pg.fsddmPending || pg.fpolkitPending)
            return;
        pg.ferr = "";
        var f = "/etc/pam.d/" + target;
        var script;
        if (on) {
            script = "f=" + f + "; "
                + "cp \"$f\" \"$f.ryoku-fp-bak\" 2>/dev/null; "
                + "grep -q pam_fprintd_grosshack \"$f\" && exit 0; "
                + "sed -i \"1i " + pg.fpamLine + "\" \"$f\"";
        } else {
            script = "f=" + f + "; "
                + "cp \"$f\" \"$f.ryoku-fp-bak\" 2>/dev/null; "
                + "sed -i '/pam_fprintd_grosshack/d' \"$f\"";
        }
        if (target === "sudo") pg.fsudoPending = true;
        else if (target === "sddm") pg.fsddmPending = true;
        else pg.fpolkitPending = true;
        pamApplyProc.target = target;
        pamApplyProc.command = ["pkexec", "bash", "-c", script];
        pamApplyProc.running = true;
    }

    readonly property string fStatusLine: {
        if (pg.floading)
            return I18n.tr("Checking\u2026");
        if (!pg.fdaemon)
            return I18n.tr("fingerprint service is not running");
        if (!pg.fready)
            return I18n.tr("no fingerprint device found");
        var parts = [ pg.fdeviceName || pg.fdevice || I18n.tr("sensor") ];
        if (pg.ffingers.length === 0)
            parts.push(I18n.tr("no fingers enrolled yet"));
        else
            parts.push(pg.ffingers.length === 1 ? I18n.tr("%1 finger enrolled").arg(pg.ffingers.length) : I18n.tr("%1 fingers enrolled").arg(pg.ffingers.length));
        return parts.join("  \u00b7  ");
    }

    // ── security key state (pam_u2f / pamu2fcfg) ───────────────────────────
    property bool skSupported: false
    property bool skDevicePresent: false
    property string skDeviceName: ""
    property bool skEnrolled: false
    property int skCredentials: 0
    property var skCredentialIds: []
    property bool skSudoOn: false
    property bool skPolkitOn: false
    property bool skLoginOn: false
    property bool skLockOn: false
    property bool skLockSupported: false
    property string skAuthMode: "either"
    property bool skTouchRequired: true
    property bool skPinVerification: false
    property bool skUserVerification: false
    property bool skLoading: true
    property string skError: ""
    property string skPending: ""
    property string skPendingTarget: ""
    property int skEnrollPolls: 0

    readonly property string skModeLine: {
        var parts = [pg.skAuthMode === "mfa" ? I18n.tr("security key + password") : I18n.tr("security key or password")];
        parts.push(pg.skTouchRequired ? I18n.tr("touch required") : I18n.tr("no touch requirement"));
        if (pg.skPinVerification)
            parts.push(I18n.tr("PIN required"));
        if (pg.skUserVerification)
            parts.push(I18n.tr("user verification required"));
        return parts.join("  \u00b7  ");
    }

    readonly property string skStatusLine: {
        if (pg.skLoading)
            return I18n.tr("Checking\u2026");
        if (!pg.skSupported)
            return I18n.tr("security-key support is unavailable");
        var parts = [];
        parts.push(pg.skDevicePresent ? (pg.skDeviceName || I18n.tr("security key detected")) : I18n.tr("no security key detected"));
        parts.push(pg.skEnrolled ? (pg.skCredentials === 1 ? I18n.tr("%1 key enrolled").arg(pg.skCredentials) : I18n.tr("%1 keys enrolled").arg(pg.skCredentials)) : I18n.tr("not enrolled yet"));
        return parts.join("  \u00b7  ");
    }

    function ffriendly(finger) {
        return pg.fnames[finger] || finger.replace(/-/g, " ");
    }

    function fheadline(mode, s) {
        var retry = {
            "retry-scan": I18n.tr("Scan didn't read cleanly \u2014 touch again"),
            "swipe-too-short": I18n.tr("Too short \u2014 hold your finger on the sensor"),
            "too-fast": I18n.tr("Too fast \u2014 slow down and press again"),
            "finger-not-centered": I18n.tr("Center your finger on the sensor"),
            "remove-and-retry": I18n.tr("Lift your finger, then touch again")
        };
        var pre = mode === "enroll" ? "enroll" : "verify";
        if (s.indexOf(pre + "-") === 0)
            s = s.slice(pre.length + 1);
        if (mode === "enroll") {
            if (s === "")
                return I18n.tr("Touch the sensor");
            if (s === "stage-passed")
                return I18n.tr("Stage captured \u2014 lift and touch again");
            if (s === "completed")
                return I18n.tr("Enrolled");
            if (s === "data-full")
                return I18n.tr("Sensor storage is full");
            if (s === "duplicate")
                return I18n.tr("That print is already on the sensor");
            if (s === "failed")
                return I18n.tr("The sensor gave up \u2014 try again");
        } else {
            if (s === "")
                return I18n.tr("Touch the sensor");
            if (s === "match")
                return I18n.tr("Match");
            if (s === "no-match")
                return I18n.tr("No match");
            if (s === "failed" || s === "unknown-error")
                return I18n.tr("Verification failed \u2014 try again");
        }
        if (s === "disconnected")
            return I18n.tr("Sensor disconnected");
        if (retry[s])
            return retry[s];
        return I18n.tr("Touch the sensor");
    }

    property bool settOpen: false

    Component.onCompleted: { pg.reload(); pg.kreload(); pg.freload(); pg.fpamReload(); pg.skreload(); }

    function kreload() {
        kstatusProc.running = true;
    }
    // pick a mode. never-ask on an encrypted keyring reveals the convert/reset
    // row instead of failing; anything else applies straight away.
    function kchoose(mode) {
        if (pg.kpending !== "")
            return;
        pg.kerror = "";
        pg.kconfirmReset = false;
        // Tested before the "already selected" guard on purpose. `keyring init`
        // records never-ask on a box whose keyring is still encrypted, so the
        // mode reads as active while nothing about it has taken effect. Bailing
        // out because the mode already matched made the row that actually fixes
        // it unreachable: the option looked chosen and the prompts kept coming.
        if (mode === "never-ask" && pg.kNeverAskBlocked) {
            pg.kconvertFor = "never-ask";
            return;
        }
        pg.kconvertFor = "";
        if (mode === pg.kmode)
            return;
        pg.kstdin = "";
        pg.kpending = mode;
        ksetProc.command = ["ryoku", "keyring", "set", mode];
        ksetProc.running = true;
    }
    function kconvert(pw) {
        if (pw.length === 0 || pg.kpending !== "")
            return;
        pg.kerror = "";
        pg.kstdin = pw + "\n";
        pg.kpending = pg.kconvertFor;
        ksetProc.command = ["ryoku", "keyring", "set", pg.kconvertFor, "--convert", "--password-stdin"];
        ksetProc.running = true;
    }
    function kreset() {
        if (pg.kpending !== "")
            return;
        pg.kerror = "";
        pg.kstdin = "";
        pg.kpending = pg.kconvertFor;
        ksetProc.command = ["ryoku", "keyring", "set", pg.kconvertFor, "--reset"];
        ksetProc.running = true;
    }

    // ── security key actions ────────────────────────────────────────────────
    function skreload() {
        pg.skLoading = true;
        skStatusProc.running = true;
    }
    function skenroll() {
        if (pg.skPending !== "")
            return;
        pg.skError = I18n.tr("Finish security-key setup in the terminal window, then return here.");
        pg.skEnrollPolls = 0;
        Quickshell.execDetached(["sh", "-c", "exec \"${TERMINAL:-kitty}\" --class ryoku-passkey -e sh -c 'ryoku security-key enroll; printf \"\\n── press enter to close ──\\n\"; read _'"]);
        skEnrollRefresh.restart();
    }
    function skremove(id) {
        if (pg.skPending !== "")
            return;
        pg.skError = "";
        pg.skPending = "remove";
        pg.skPendingTarget = id;
        skActionProc.command = ["ryoku", "security-key", "remove", id];
        skActionProc.running = true;
    }
    function sktoggle(target, on) {
        if (pg.skPending !== "")
            return;
        pg.skError = "";
        pg.skPending = "toggle";
        pg.skPendingTarget = target;
        skActionProc.command = ["ryoku", "security-key", "set", target, on ? "on" : "off"];
        skActionProc.running = true;
    }
    function sksetMode(mode) {
        if (pg.skPending !== "")
            return;
        pg.skError = "";
        pg.skPending = "mode";
        pg.skPendingTarget = mode;
        skActionProc.command = ["ryoku", "security-key", "set", "mode", mode];
        skActionProc.running = true;
    }
    function sksetFlag(name, on) {
        if (pg.skPending !== "")
            return;
        pg.skError = "";
        pg.skPending = "flag";
        pg.skPendingTarget = name;
        skActionProc.command = ["ryoku", "security-key", "set", name, on ? "on" : "off"];
        skActionProc.running = true;
    }

    // ── fingerprint actions ─────────────────────────────────────────────────
    function freload() {
        pg.floading = true;
        fpTimeout.restart();
        freadProc.running = true;
        flistProc.running = true;
        fnamesReadProc.running = true;
        pg.fpamReload();
    }
    function fparseList(text) {
        fpTimeout.stop();
        var t = text || "";
        var dev = "";
        var m = t.match(/Using\s+device\s+(\S+)/i);
        if (m) dev = m[1];
        var dn = t.match(/on (.+?) \((?:press|swipe)\):/i);
        pg.fdeviceName = dn ? dn[1] : "";
        var fingers = [];
        var lines = t.split("\n");
        for (var i = 0; i < lines.length; i++) {
            var s = lines[i].trim();
            if (s.indexOf("- #") === 0) {
                var idx = s.indexOf(":", 3);
                if (idx > -1) fingers.push(s.slice(idx + 1).trim());
            }
        }
        pg.fdevice = dev;
        pg.ffingers = fingers;
        // A reader is present only when fprintd reports an actual device. The
        // header "found N devices" is printed even when N is 0 (no hardware),
        // so gate on the count / a "Using device" line -- never a bare "found ".
        var fm = t.match(/found\s+(\d+)\s+devices?/i);
        var nDev = fm ? parseInt(fm[1], 10) : 0;
        pg.fready = t.indexOf("Using device") !== -1 || t.indexOf("Device at") !== -1 || nDev > 0;
        pg.fdaemon = pg.fready && t.trim() !== "" && t.indexOf("no devices") === -1;
        pg.floading = false;
        if (pg.fdaemon && !pg.floading) {
            var stale = false;
            for (var k in pg.fnames)
                if (fingers.indexOf(k) === -1) { delete pg.fnames[k]; stale = true; }
            if (stale) pg.fwriteNames();
        }
    }
    function ftoggle(v) {
        pg.ffpEnabled = v;
        fwriteProc.command = [
            "bash", "-c",
            "mkdir -p \"$HOME/.config/qylock\"; printf '%s\\n' " + (v ? "on" : "off") + " > \"$HOME/.config/qylock/fingerprint\""
        ];
        fwriteProc.running = true;
    }
    function fparseNames(text) {
        try {
            pg.fnames = JSON.parse(text || "{}");
        } catch (e) {
            pg.fnames = {};
        }
    }
    function fwriteNames() {
        var json = JSON.stringify(pg.fnames);
        fnamesWriteProc.command = ["bash", "-c", "mkdir -p \"$HOME/.config/qylock\"; cat > \"$HOME/.config/qylock/fingerprints.json\""];
        fnamesWriteProc.stdinEnabled = true;
        fnamesWriteProc.running = true;
        // Write happens in onStarted
    }
    function fopenOverlay(mode, cmd, proc) {
        pg.ferr = "";
        pg.fresult = "";
        pg.fstagesPassed = 0;
        pg.fstagesTotal = 0;
        pg.fterm = "";
        pg.fstatus = "";
        pg.foffsets = {};
        pg.fpending = mode;
        pg.foverlayMode = mode;
        pg.fnaming = false;
        pg.fnewFinger = "";
        pg.fnameDraft = "";
        pg.foverlay = true;
        proc.command = cmd;
        proc.running = true;
    }
    function fstartEnroll() {
        if (pg.fpending !== "" || !pg.fdaemon || !pg.fready)
            return;
        pg.fopenOverlay("enroll", ["fprintd-enroll"], fenrollProc);
        fstagesProc.running = true;
        stagesRetry.tries = 0;
        stagesRetry.restart();
    }
    function fstartVerify(finger) {
        if (pg.fpending !== "" || !pg.fdaemon || !pg.fready)
            return;
        pg.fopenOverlay("verify", finger === "" ? ["fprintd-verify"] : ["fprintd-verify", "-f", finger], fverifyProc);
    }
    function fdelete(finger) {
        if (pg.fpending !== "")
            return;
        pg.ferr = "";
        pg.fdelTarget = finger;
        pg.fpending = "del";
        fdelProc.command = ["fprintd-delete", pg.fuser].concat(finger ? ["-f", finger] : []);
        fdelProc.running = true;
    }
    // pump fresh bytes of one stream; counts each stage exactly once.
    function fpump(stream, full) {
        full = full || "";
        var start = pg.foffsets[stream] || 0;
        if (full.length <= start)
            return;
        var chunk = full.substring(start);
        pg.foffsets[stream] = full.length;
        pg.fappendTerm(chunk);
        var m, re = /(?:Enroll result|Verify result):\s*([a-z0-9-]+)/gi;
        while ((m = re.exec(chunk)) !== null) {
            pg.fstatus = m[1].toLowerCase();
            if (m[1] === "enroll-stage-passed")
                pg.fstagesPassed++;
        }
    }
    function fcloseOverlay() {
        verifyClose.stop();
        if (fenrollProc.running)
            fenrollProc.signal(15);
        if (fverifyProc.running)
            fverifyProc.signal(15);
        pg.fpending = "";
        pg.foverlay = false;
        pg.foverlayMode = "";
        pg.fstatus = "";
        pg.fresult = "";
        pg.fterm = "";
        pg.fstagesPassed = 0;
        pg.fstagesTotal = 0;
        pg.foffsets = {};
        pg.fnaming = false;
        pg.fnewFinger = "";
        pg.fnameDraft = "";
        // every way out of the modal lands here, so this is the one place
        // that refreshes the finger list.
        pg.freload();
    }
    function fappendTerm(raw) {
        var t = (raw || "").trim();
        if (!t) return;
        // Strip "Using device ..." lines, keep the rest
        var lines = t.split("\n");
        var out = [];
        for (var i = 0; i < lines.length; i++) {
            var s = lines[i].trim();
            if (s && !s.match(/^Using device/i))
                out.push(s);
        }
        if (out.length > 0)
            pg.fterm += out.join("\n") + "\n";
    }
    function fsaveName() {
        if (pg.fnewFinger !== "" && pg.fnameDraft.trim() !== "") {
            pg.fnames[pg.fnewFinger] = pg.fnameDraft.trim();
            pg.fwriteNames();
        }
        pg.fcloseOverlay();
    }
    function fskipName() {
        if (pg.fnewFinger !== "") {
            pg.fnames[pg.fnewFinger] = pg.fnewFinger.replace(/-/g, " ");
            pg.fwriteNames();
        }
        pg.fcloseOverlay();
    }
    readonly property var ftail: {
        var lines = pg.fterm.trim().split("\n");
        while (lines.length < 2) lines.unshift("");
        return lines.slice(-2);
    }

    function browseStore() {
        Quickshell.execDetached(["ryostore", "open", "lockscreens"]);
    }
    function reload() {
        pg.loading = pg.skins.length === 0;
        pg.loadFailed = false;
        listProc.command = ["ryoku-hub", "lock", "list"];
        listProc.running = true;
    }
    function select(skin) {
        if (skin.slug === pg.active || pg.pendingSlug !== "")
            return;
        pg.error = "";
        pg.pendingSlug = skin.slug;
        actProc.command = ["ryoku-hub", "lock", "set", skin.slug];
        actProc.running = true;
    }
    function preview(slug) {
        Spawn.run([pg.lockLauncher, slug]);
    }

    // live filter: name, theme, slug, tags and copy all match the rail query.
    readonly property var shown: {
        var q = pg.query.trim().toLowerCase();
        if (q === "")
            return pg.skins;
        var out = [];
        for (var i = 0; i < pg.skins.length; i++) {
            var s = pg.skins[i];
            var hay = ((s.name || "") + " " + (s.theme || "") + " " + (s.slug || "") + " "
                + (s.summary || "") + " " + (s.blurb || "") + " " + ((s.tags || []).join(" "))).toLowerCase();
            if (hay.indexOf(q) !== -1)
                out.push(s);
        }
        return out;
    }

    // greedy masonry like the old grid: each tile drops into the shortest
    // column, its height estimated from the hero plus blurb length so the
    // columns stay balanced.
    function buildColumns(list, n) {
        var c = [], h = [], i;
        for (i = 0; i < n; i++) { c.push([]); h.push(0); }
        for (i = 0; i < list.length; i++) {
            var est = 300 + Math.ceil(((list[i].blurb || "").length) / 30) * 16;
            var min = 0;
            for (var j = 1; j < n; j++)
                if (h[j] < h[min]) min = j;
            c[min].push(list[i]);
            h[min] += est + Tokens.s3;
        }
        return c;
    }
    readonly property var grouped: pg.buildColumns(pg.shown, pg.cols)

    // ── installed themes from Ryoku Hub ─────────────────────────────────────
    Process {
        id: listProc
        command: ["ryoku-hub", "lock", "list"]
        stdout: StdioCollector { id: listOut }
        stderr: StdioCollector { id: listErr }
        // A non-zero exit (ryoku-hub missing from the Hub's PATH, a crash), a
        // backend that reports a read error, or unparsable output is a listing
        // FAILURE, kept distinct from a valid empty list so the page never says
        // "nothing installed" when it simply couldn't ask. StdioCollector waits
        // for stream end, so the collected text is complete in onExited.
        onExited: code => {
            if (code !== 0) {
                pg.skins = [];
                pg.loadFailed = true;
                pg.loading = false;
                return;
            }
            try {
                const response = JSON.parse(listOut.text || "{}");
                if (response.error) {
                    pg.skins = [];
                    pg.loadFailed = true;
                } else {
                    pg.skins = (response.skins || []).map((skin, index) => ({
                        slug: skin.slug,
                        name: skin.name || skin.slug,
                        theme: skin.theme || "",
                        preview: skin.preview || "",
                        summary: skin.summary || "",
                        blurb: skin.blurb || "",
                        tags: skin.tags || [],
                        active: skin.active === true,
                        installed: true,
                        ordinal: index + 1
                    }));
                    pg.active = response.active || "";
                    pg.loadFailed = false;
                }
            } catch (e) {
                pg.skins = [];
                pg.loadFailed = true;
            }
            pg.loading = false;
        }
    }
    Process {
        id: actProc
        stderr: StdioCollector { id: actErr }
        onExited: code => {
            if (code !== 0)
                pg.error = I18n.tr("Couldn't switch skin: %1").arg(actErr.text.trim() || ("exit " + code));
            pg.pendingSlug = "";
            pg.reload();
        }
    }

    // ── sign-in keyring backend ─────────────────────────────────────────────
    Process {
        id: kstatusProc
        command: ["ryoku", "keyring", "status", "--json"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var o = JSON.parse(this.text);
                    pg.kmode = o.mode || "";
                    pg.kdaemon = o.daemon_alive === true;
                    pg.knotes = o.notes || [];
                    var def = null;
                    var ks = o.keyrings || [];
                    for (var i = 0; i < ks.length; i++)
                        if (ks[i].role === "default")
                            def = ks[i];
                    pg.kdefName = def ? def.name : "";
                    pg.kdefFormat = def ? def.format : "";
                    // never-ask on paper, encrypted on disk: the mode cannot take
                    // effect, so open the convert/reset row without waiting for a
                    // click on an option that already looks selected.
                    if (pg.kmode === "never-ask" && pg.kdefFormat === "encrypted" && pg.kpending === "")
                        pg.kconvertFor = "never-ask";
                } catch (e) {
                    pg.kerror = I18n.tr("Couldn't read the keyring status.");
                }
                pg.kloading = false;
            }
        }
    }
    Process {
        id: ksetProc
        stdinEnabled: true
        stderr: StdioCollector { id: ksetErr }
        onStarted: {
            if (pg.kstdin.length > 0) {
                write(pg.kstdin);
                pg.kstdin = "";
            }
        }
        onExited: (code) => {
            pg.kpending = "";
            if (code !== 0) {
                pg.kerror = ksetErr.text.trim() || ("exit " + code);
            } else {
                pg.kconvertFor = "";
                pg.kconfirmReset = false;
            }
            pg.kreload();
        }
    }

    // ── security key backend (pam_u2f / pamu2fcfg) ─────────────────────────
    Process {
        id: skStatusProc
        command: ["ryoku", "security-key", "status", "--json"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var o = JSON.parse(this.text);
                    pg.skSupported = o.supported === true;
                    pg.skDevicePresent = o.devicePresent === true;
                    pg.skDeviceName = o.deviceName || "";
                    pg.skEnrolled = o.enrolled === true;
                    pg.skCredentials = o.credentials || 0;
                    pg.skCredentialIds = o.credentialIds || [];
                    pg.skSudoOn = o.sudo === true;
                    pg.skPolkitOn = o.polkit === true;
                    pg.skLoginOn = o.login === true;
                    pg.skLockOn = o.lock === true;
                    pg.skLockSupported = o.lockSupported === true;
                    pg.skAuthMode = o.authMode || "either";
                    pg.skTouchRequired = o.touchRequired !== false;
                    pg.skPinVerification = o.pinVerification === true;
                    pg.skUserVerification = o.userVerification === true;
                    pg.skError = "";
                } catch (e) {
                    pg.skError = I18n.tr("Couldn't read the security-key status.");
                }
                pg.skLoading = false;
            }
        }
    }
    Process {
        id: skActionProc
        stdout: StdioCollector { id: skActionOut }
        stderr: StdioCollector { id: skActionErr }
        onExited: (code) => {
            pg.skPending = "";
            pg.skPendingTarget = "";
            if (code !== 0)
                pg.skError = skActionErr.text.trim() || skActionOut.text.trim() || ("exit " + code);
            else
                pg.skError = "";
            pg.skreload();
        }
    }
    Timer {
        id: skEnrollRefresh
        interval: 2500
        repeat: true
        onTriggered: {
            pg.skEnrollPolls++;
            pg.skreload();
            if (pg.skEnrolled || pg.skEnrollPolls >= 24)
                stop();
        }
    }

    // ── fingerprint backend (fprintd as this user; no root) ──────────────────
    Process {
        id: freadProc
        command: [ "bash", "-c", "[ -f \"$HOME/.config/qylock/fingerprint\" ] && cat \"$HOME/.config/qylock/fingerprint\" || printf 'on'" ]
        stdout: StdioCollector { id: freadOut }
        onExited: () => {
            var v = freadOut.text.trim().toLowerCase();
            pg.ffpEnabled = !(v === "off" || v === "0" || v === "false");
        }
    }
    Process {
        id: flistProc
        command: ["fprintd-list", pg.fuser]
        stdout: StdioCollector { id: flistOut }
        onExited: (code) => {
            if (code !== 0 && flistOut.text.trim() === "") {
                pg.fdaemon = false; pg.fready = false; pg.ffingers = []; pg.floading = false; fpTimeout.stop();
                pg.ferr = I18n.tr("The fingerprint service is not running.");
                return;
            }
            pg.ferr = "";
            pg.fparseList(flistOut.text);
        }
    }
    // fprintd is D-Bus activated; a probe can hang if the service never comes
    // up. Don't sit on "Checking…" forever -- fail to a clear, honest state.
    Timer {
        id: fpTimeout
        interval: 6000
        onTriggered: {
            if (!pg.floading)
                return;
            pg.floading = false;
            pg.fdaemon = false;
            pg.fready = false;
            pg.ffingers = [];
            pg.ferr = I18n.tr("The fingerprint service isn't responding.");
        }
    }
    Process {
        id: fwriteProc
        onExited: () => { pg.freload(); }
    }
    Process {
        id: fnamesReadProc
        command: ["bash", "-c", "[ -f \"$HOME/.config/qylock/fingerprints.json\" ] && cat \"$HOME/.config/qylock/fingerprints.json\" || printf '{}'" ]
        stdout: StdioCollector { id: fnamesReadOut }
        onExited: () => { pg.fparseNames(fnamesReadOut.text); }
    }
    Process {
        id: fnamesWriteProc
        stdinEnabled: true
        onStarted: {
            if (pg.fnames !== undefined) {
                write(JSON.stringify(pg.fnames));
            }
        }
        onExited: () => { pg.freload(); }
    }
    // both streams pump through fpump (stdout carries "Enroll result:").
    Process {
        id: fenrollProc
        stdout: StdioCollector { id: fenOut; waitForEnd: false; onDataChanged: pg.fpump("out", this.text) }
        stderr: StdioCollector { id: fenErr; waitForEnd: false; onDataChanged: pg.fpump("err", this.text) }
        onExited: (code) => {
            stagesRetry.stop();
            pg.fappendTerm("\n--- " + (code === 0 ? "DONE" : "EXIT " + code) + " ---\n");
            var t = (fenOut.text || "") + "\n" + (fenErr.text || "");
            pg.fpending = "";
            if (/enroll-completed/i.test(t)) {
                // fprintd prints the finger key either hyphenated or spaced;
                // normalise back to the D-Bus form fprintd-list reports.
                var m = t.match(/Enrolling\s+\"?([a-z0-9\- ]*finger)\"?\.?/i);
                if (m && /finger/i.test(m[1])) {
                    pg.fnewFinger = m[1].trim().replace(/\s+/g, "-").toLowerCase();
                    pg.fnameDraft = pg.fnewFinger.replace(/-/g, " ");
                    pg.fnaming = true;
                    pg.fresult = I18n.tr("Enrollment complete. Name this fingerprint.");
                    return;
                }
                pg.fresult = I18n.tr("Fingerprint enrolled.");
            } else if (/enroll-data-full/i.test(t)) {
                pg.fresult = I18n.tr("Sensor storage is full.");
            } else if (/enroll-duplicate|enroll-failed|enroll-disconnected|enroll-unknown-error/i.test(t)) {
                pg.fresult = I18n.tr("Enrollment did not finish \u2014 try again.");
            } else if (code !== 0) {
                pg.fresult = I18n.tr("Enrollment stopped.");
            }
            pg.freload();
        }
    }
    Process {
        id: fstagesProc
        command: [
            "busctl", "call", "net.reactivated.Fprint",
            "/net/reactivated/Fprint/Device/0",
            "org.freedesktop.DBus.Properties", "Get",
            "ss", "net.reactivated.Fprint.Device", "num-enroll-stages"
        ]
        stdout: StdioCollector { id: fstagesOut }
        onExited: (code) => {
            var m = (fstagesOut.text || "").match(/i\s+(\d+)/);
            if (code === 0 && m)
                pg.fstagesTotal = parseInt(m[1]);
        }
    }
    Timer {
        id: stagesRetry
        interval: 1500
        property int tries: 0
        onTriggered: {
            if (pg.fpending !== "enroll" || pg.fstagesTotal > 0 || tries >= 3)
                return;
            tries++;
            fstagesProc.running = true;
            restart();
        }
    }
    Process {
        id: fverifyProc
        stdout: StdioCollector { id: fverOut; waitForEnd: false; onDataChanged: pg.fpump("out", this.text) }
        stderr: StdioCollector { id: fverErr; waitForEnd: false; onDataChanged: pg.fpump("err", this.text) }
        onExited: (code) => {
            pg.fappendTerm("\n--- " + (code === 0 ? "MATCH" : "NO MATCH") + " ---\n");
            pg.fpending = "";
            var t = (fverOut.text || "") + "\n" + (fverErr.text || "");
            // fprintd-verify can exit 0 while printing no-match on some
            // versions; trust the status token over the exit code.
            if (!/verify-no-match/i.test(t) && (code === 0 || /verify-match/i.test(t))) {
                pg.fstatus = "verify-match";
                pg.fresult = I18n.tr("Fingerprint verified.");
                verifyClose.restart();
            } else {
                pg.fstatus = "verify-no-match";
                pg.fresult = I18n.tr("No match.");
            }
        }
    }
    Timer { id: verifyClose; interval: 1400; onTriggered: pg.fcloseOverlay() }
    Process {
        id: fdelProc
        stderr: StdioCollector { id: fdelErr }
        onExited: (code) => {
            if (code !== 0) {
                pg.ferr = fdelErr.text.trim() || (I18n.tr("Couldn't remove the fingerprint") + " (exit " + code + ")");
            } else {
                // a named delete drops its label too; REMOVE ALL clears the
                // whole map. fwriteNames is the single reload path here.
                if (pg.fdelTarget !== "")
                    delete pg.fnames[pg.fdelTarget];
                else
                    pg.fnames = {};
                pg.fwriteNames();
            }
            pg.fdelTarget = "";
            pg.frmConfirm = false;
            pg.fdelConfirmFor = "";
            pg.fpending = "";
        }
    }
    Timer {
        id: confirmReset
        interval: 3000
        onTriggered: { pg.frmConfirm = false; pg.fdelConfirmFor = ""; }
    }

    // ── grosshack elsewhere: status of /etc/pam.d/{sudo,sddm} ────────────────
    Process {
        id: pamStatusProc
        command: ["bash", "-c",
            "printf 'module='; [ -f /usr/lib/security/pam_fprintd_grosshack.so ] && echo 1 || echo 0; "
            + "for f in sudo sddm polkit-1; do printf '%s=' \"$f\"; "
            + "grep -qs pam_fprintd_grosshack \"/etc/pam.d/$f\" && echo 1 || echo 0; done"]
        stdout: StdioCollector { id: pamStatusOut }
        onExited: () => {
            var o = {};
            (pamStatusOut.text || "").split("\n").forEach(function (line) {
                var kv = line.split("=");
                if (kv.length === 2) o[kv[0]] = kv[1].trim();
            });
            pg.fpamModuleOk = o["module"] === "1";
            pg.fsudoOn = o["sudo"] === "1";
            pg.fsddmOn = o["sddm"] === "1";
            pg.fpolkitOn = o["polkit-1"] === "1";
            pg.fpamLoading = false;
        }
    }
    Process {
        id: pamApplyProc
        property string target: ""
        stderr: StdioCollector { id: pamApplyErr }
        onExited: (code) => {
            if (pamApplyProc.target === "sudo") pg.fsudoPending = false;
            else if (pamApplyProc.target === "sddm") pg.fsddmPending = false;
            else pg.fpolkitPending = false;
            if (code !== 0) {
                var t = pamApplyProc.target === "sudo" ? I18n.tr("sudo")
                    : (pamApplyProc.target === "sddm" ? I18n.tr("the sign-in screen") : I18n.tr("admin prompts"));
                pg.ferr = (pamApplyErr.text.trim() !== "")
                    ? I18n.tr("Couldn't update %1: %2").arg(t).arg(pamApplyErr.text.trim())
                    : I18n.tr("Couldn't update %1 (cancelled?)").arg(t);
            } else {
                pg.ferr = "";
            }
            pg.fpamReload();
        }
    }

    // ── head: eyebrow, Fraunces title + refresh, blurb, error line ──────────
    Column {
        id: head
        anchors.top: parent.top
        anchors.topMargin: Tokens.s6
        // the head sits on the body's grid, so the title starts over the first
        // card column instead of floating in the middle of a page-wide window
        x: Tokens.s6
        width: Math.max(320, pg.width - Tokens.s6 * 2 - Tokens.s3)
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "力"; color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("DESKTOP"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        // title with its one utility action (rescan the installed skins) beside it.
        Item {
            width: parent.width
            height: title.height
            Text {
                id: title
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Lockscreen")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle
            }
            Btn {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("BROWSE RYOSTORE")
                onAct: pg.browseStore()
            }
        }

        Text {
            width: Math.min(parent.width, 720)
            // the load-bearing caveat, kept verbatim: it only swaps the look,
            // never your login.
            text: I18n.tr("Pick the skin your lock and sign-in screens wear. It only swaps the look, never your login; applying reskins the sign-in screen too, so you will be asked for your password.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
        Text {
            width: Math.min(parent.width, 720)
            visible: pg.error !== ""
            // no red on the sheet: an error is the brightest ink and the word.
            text: pg.error
            color: Tokens.ink; font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; wrapMode: Text.WordWrap
        }
    }

    // ── loading / none-installed / read-failure state ───────────────────────
    // Three outcomes share one centred column: a spinner while loading, a Store
    // nudge when the list came back empty, and a retry when it couldn't be read.
    Column {
        anchors.centerIn: parent
        visible: pg.loading || pg.loadFailed || pg.emptyInstalled
        spacing: Tokens.s3
        width: Math.min(pg.width - Tokens.s6 * 2, 420)

        Glyph {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: pg.loading
            path: pg.pRefresh; size: 26; weight: 2; tint: Tokens.inkMuted
            RotationAnimator on rotation {
                from: 0; to: 360; duration: 900; loops: Animation.Infinite; running: pg.loading
            }
        }
        Glyph {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: pg.loadFailed || pg.emptyInstalled
            path: pg.pLock; size: 44; tint: Tokens.inkFaint
        }
        Text {
            width: parent.width
            visible: pg.loadFailed
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
            text: I18n.tr("Couldn't read the lock skins list. Try again.")
            color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
        }
        Text {
            width: parent.width
            visible: pg.emptyInstalled
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
            text: I18n.tr("No lock skins installed yet. Get some from Ryoku Store.")
            color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
        }
        Btn {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: pg.loadFailed
            text: I18n.tr("TRY AGAIN")
            onAct: pg.reload()
        }
        Btn {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: pg.emptyInstalled
            text: I18n.tr("BROWSE STORE")
            onAct: pg.browseStore()
        }
    }

    // ── no-matches state (a search that filtered everything out) ────────────
    Text {
        anchors.centerIn: parent
        visible: !pg.loading && !pg.loadFailed && pg.shown.length === 0 && pg.query.trim() !== "" && pg.skins.length > 0
        text: I18n.tr("No skins match your search.")
        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
    }

    // ── Sign-in & Fingerprint: combined collapsible settings card ──────────────
    Rectangle {
        id: sett
        property bool open: false
        anchors { left: parent.left; right: parent.right; top: head.bottom }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.topMargin: Tokens.s4
        implicitHeight: settCol.implicitHeight + Tokens.s4 * 2
        height: implicitHeight
        radius: Tokens.radius
        color: "transparent"
        border.width: Tokens.border
        border.color: Tokens.line

        Column {
            id: settCol
            anchors { left: parent.left; right: parent.right; top: parent.top }
            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s4
            spacing: Tokens.s2

            // header with collapse chevron riding beside the title (not the
            // far edge, where it read as decoration instead of the control)
            Row {
                width: parent.width
                spacing: Tokens.s2
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Sign-in, Fingerprint & Security key")
                    color: Tokens.ink; font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow; font.weight: Font.DemiBold
                }
                Glyph {
                    anchors.verticalCenter: parent.verticalCenter
                    path: pg.pChevron; size: 15; weight: 2; rotation: pg.settOpen ? 90 : -90
                    tint: Tokens.ink
                    Behavior on rotation { NumberAnimation { duration: Tokens.snap } }
                }
                Item { width: Tokens.s1; height: 1 }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.max(0, parent.width - x)
                    text: pg.settOpen ? "" : (pg.kStatusLine + "  \u00b7  " + pg.fStatusLine)
                    horizontalAlignment: Text.AlignRight
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; elide: Text.ElideRight
                }
                TapHandler { onTapped: pg.settOpen = !pg.settOpen }
            }

            Row {
                visible: pg.settOpen
                spacing: Tokens.s5

                // ── left: keyring, sensor, switches ──
                Flickable {
                    id: settingsLeft
                    width: Math.round((settCol.width - Tokens.s5 - Tokens.border) * 0.56)
                    height: Math.min(settingsLeftCol.implicitHeight, Math.max(260, pg.height - sett.y - 180))
                    contentWidth: width
                    contentHeight: settingsLeftCol.implicitHeight
                    clip: true
                    boundsBehavior: Flickable.StopAtBounds
                    ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                    WheelScroll { }

                    Column {
                        id: settingsLeftCol
                        width: settingsLeft.width - Tokens.s3
                        spacing: Tokens.s3

                // ── Keyring section ──
                Column {
                    width: parent.width
                    spacing: Tokens.s2

                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        Text { text: I18n.tr("Keyring"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.capitalization: Font.AllUppercase; font.letterSpacing: Tokens.trackMark }
                    }

                    // three-mode chip row.
                    Row { topPadding: Tokens.s1; spacing: Tokens.s2
                        Chip { label: I18n.tr("Unlock at sign-in"); mode: "unlock-on-login" }
                        Chip { label: I18n.tr("Never ask"); mode: "never-ask" }
                        Chip { label: I18n.tr("Ask each time"); mode: "ask" }
                    }

                    Text { width: parent.width; topPadding: Tokens.s1; text: pg.kStatusLine; color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap }
                    Text { width: parent.width; visible: pg.knotes.length > 0 && pg.kconvertFor === ""; text: pg.knotes.join("\n"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap; lineHeight: 1.3 }

                    Column {
                        width: parent.width; visible: pg.kconvertFor !== ""; topPadding: Tokens.s2; spacing: Tokens.s2
                        Text { width: parent.width; text: I18n.tr("That keyring is locked with a password. Enter it to switch it to no-password, or start fresh (your old keyring is backed up, never deleted)."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap }
                        Row { spacing: Tokens.s2
                            Rectangle { anchors.verticalCenter: parent.verticalCenter; width: 240; height: Tokens.ctlH + 4; radius: Tokens.radius; color: "transparent"; border.width: Tokens.border; border.color: kpwField.activeFocus ? Tokens.ink : Tokens.line; Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
                                TextInput { id: kpwField; anchors.fill: parent; anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3; verticalAlignment: TextInput.AlignVCenter; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; echoMode: TextInput.Password; selectByMouse: true; selectionColor: Tokens.ink; selectedTextColor: Tokens.inkOnBone; onAccepted: pg.kconvert(text)
                                    Text { anchors { left: parent.left; verticalCenter: parent.verticalCenter } visible: kpwField.text.length === 0; text: I18n.tr("Current keyring password"); color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                }
                            }
                            Btn { anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("CONVERT"); compact: true; armed: pg.kpending === "" && kpwField.text.length > 0; onAct: pg.kconvert(kpwField.text) }
                            Btn { anchors.verticalCenter: parent.verticalCenter; text: pg.kconfirmReset ? I18n.tr("CONFIRM - START FRESH") : I18n.tr("START FRESH (KEEPS A BACKUP)"); compact: true; armed: pg.kpending === ""; onAct: { if (pg.kconfirmReset) pg.kreset(); else pg.kconfirmReset = true; } }
                        }
                    }
                    Text { width: parent.width; visible: pg.kerror !== ""; topPadding: Tokens.s1; text: pg.kerror; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; wrapMode: Text.WordWrap }
                }

                // hairline separator
                Rectangle { width: parent.width; height: Tokens.border; color: Tokens.line; visible: pg.settOpen && pg.ffingers.length >= 0 }

                // ── fingerprint: sensor & switches ──
                Column {
                    width: parent.width
                    spacing: Tokens.s2

                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        Text { text: I18n.tr("Fingerprint"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.capitalization: Font.AllUppercase; font.letterSpacing: Tokens.trackMark }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - x
                            text: pg.fStatusLine
                            horizontalAlignment: Text.AlignRight
                            color: Tokens.inkFaint; font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny; elide: Text.ElideRight
                        }
                    }

                    // sensor identity row: who is listening, and is anyone.
                    Item {
                        width: parent.width
                        height: Math.max(sensorGlyph.height, sensorCol.implicitHeight)

                        Glyph {
                            id: sensorGlyph
                            anchors { left: parent.left; top: parent.top; topMargin: 2 }
                            path: pg.pFingerprint; size: 22; weight: 1.5
                            tint: pg.fready ? Tokens.ink : Tokens.inkFaint
                        }
                        Column {
                            id: sensorCol
                            anchors { left: sensorGlyph.right; right: sensorRetry.visible ? sensorRetry.left : parent.right; leftMargin: Tokens.s3; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                            spacing: 2

                            Text {
                                width: parent.width
                                text: !pg.floading ? (pg.fdeviceName || (pg.fready ? I18n.tr("Fingerprint sensor") : I18n.tr("No fingerprint sensor"))) : I18n.tr("Checking\u2026")
                                color: pg.fready ? Tokens.ink : Tokens.inkMuted
                                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.DemiBold
                                elide: Text.ElideRight
                            }
                            Text {
                                width: parent.width
                                text: {
                                    if (pg.floading)
                                        return "";
                                    if (!pg.fdaemon)
                                        return I18n.tr("The fingerprint service is not running. Enroll anyway and it will start on demand.");
                                    if (!pg.fready)
                                        return I18n.tr("No sensor found. Check the USB connection, then retry.");
                                    if (pg.ffingers.length === 0)
                                        return I18n.tr("Ready. Enroll a finger to unlock with a touch.");
                                    return pg.ffpEnabled ? I18n.tr("Listening at the lock screen") : I18n.tr("Listening at the lock screen \u00b7 currently switched off");
                                }
                                color: Tokens.inkDim; font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                            }
                        }
                        Btn {
                            id: sensorRetry
                            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                            visible: !pg.floading && (!pg.fdaemon || !pg.fready)
                            text: I18n.tr("RETRY"); compact: true
                            onAct: pg.freload()
                        }
                    }

                    // hairline
                    Rectangle { width: parent.width; height: Tokens.border; color: Tokens.lineSoft }

                    // toggle: label + consequence on the left, control pinned right.
                    Item {
                        width: parent.width
                        height: Math.max(toggleCol.height, tog.implicitHeight)
                        // A stored preference never needs hardware: stay operable
                        // even with no sensor so it can always be switched off.
                        enabled: !pg.floading

                        Column {
                            id: toggleCol
                            anchors { left: parent.left; right: tog.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                            spacing: 2
                            Text { text: I18n.tr("Unlock with fingerprint"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                            Text {
                                width: parent.width
                                text: !pg.ffpEnabled ? I18n.tr("Password only")
                                    : (pg.fready ? I18n.tr("The lock screen listens for a touch before asking your password")
                                                 : I18n.tr("On, but no fingerprint sensor is connected"))
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                wrapMode: Text.WordWrap
                            }
                        }
                        Sw {
                            id: tog
                            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                            on: pg.ffpEnabled
                            onToggled: (v) => pg.ftoggle(v)
                        }
                    }

                    // grosshack for sudo / sddm; root half runs via pkexec.
                    Column {
                        visible: pg.fpamModuleOk && !pg.fpamLoading
                        width: parent.width
                        spacing: Tokens.s2

                        Rectangle { width: parent.width; height: Tokens.border; color: Tokens.lineSoft }

                        Item {
                            width: parent.width
                            height: Math.max(sudoCol.height, sudoSw.implicitHeight)
                            Column {
                                id: sudoCol
                                anchors { left: parent.left; right: sudoSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                spacing: 2
                                Text { text: I18n.tr("Sudo"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                Text {
                                    width: parent.width
                                    text: I18n.tr("Touch instead of typing for admin commands")
                                    color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    wrapMode: Text.WordWrap
                                }
                            }
                            Sw {
                                id: sudoSw
                                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                opacity: pg.fsudoPending ? 0.4 : 1
                                Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                on: pg.fsudoOn
                                onToggled: (v) => pg.fpamToggle("sudo", v)
                            }
                        }

                        Item {
                            width: parent.width
                            height: Math.max(sddmCol.height, sddmSw.implicitHeight)
                            Column {
                                id: sddmCol
                                anchors { left: parent.left; right: sddmSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                spacing: 2
                                Text { text: I18n.tr("Sign-in screen"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                Text {
                                    width: parent.width
                                    text: I18n.tr("Fingerprint at the greeter too (SDDM)")
                                    color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    wrapMode: Text.WordWrap
                                }
                            }
                            Sw {
                                id: sddmSw
                                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                opacity: pg.fsddmPending ? 0.4 : 1
                                Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                on: pg.fsddmOn
                                onToggled: (v) => pg.fpamToggle("sddm", v)
                            }
                        }

                        Item {
                            width: parent.width
                            height: Math.max(polkitCol.height, polkitSw.implicitHeight)
                            Column {
                                id: polkitCol
                                anchors { left: parent.left; right: polkitSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                spacing: 2
                                Text { text: I18n.tr("Admin prompts"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                Text {
                                    width: parent.width
                                    text: I18n.tr("Touch for the pop-up admin question (polkit)")
                                    color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    wrapMode: Text.WordWrap
                                }
                            }
                            Sw {
                                id: polkitSw
                                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                opacity: pg.fpolkitPending ? 0.4 : 1
                                Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                on: pg.fpolkitOn
                                onToggled: (v) => pg.fpamToggle("polkit-1", v)
                            }
                        }

                        // hairline
                        Rectangle { width: parent.width; height: Tokens.border; color: Tokens.lineSoft }
                    }

                    // ── security key: enroll & PAM switches ──
                    Column {
                        width: parent.width
                        spacing: Tokens.s2

                        Rectangle { width: parent.width; height: Tokens.border; color: Tokens.line }

                        Row {
                            width: parent.width
                            spacing: Tokens.s2
                            Text { text: I18n.tr("Security key / Passkey"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.capitalization: Font.AllUppercase; font.letterSpacing: Tokens.trackMark }
                        }

                        Text {
                            width: parent.width
                            text: I18n.tr("Choose where enrolled passkeys are accepted and how they authenticate.")
                            color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }

                        Rectangle { width: parent.width; height: Tokens.border; color: Tokens.lineSoft }

                        Column {
                            width: parent.width
                            spacing: Tokens.s2
                            Text {
                                width: parent.width
                                text: I18n.tr("Use passkey for")
                                color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            }

                            Item {
                                width: parent.width
                                height: Math.max(skSudoCol.height, skSudoSw.implicitHeight)
                                visible: pg.skSupported
                                Column {
                                    id: skSudoCol
                                    anchors { left: parent.left; right: skSudoSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                    spacing: 2
                                    Text { text: I18n.tr("Sudo"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                    Text {
                                        width: parent.width
                                        text: I18n.tr("Use your security key for terminal admin commands")
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                        wrapMode: Text.WordWrap
                                    }
                                }
                                Sw {
                                    id: skSudoSw
                                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                    opacity: pg.skPending === "toggle" && pg.skPendingTarget === "sudo" ? 0.4 : 1
                                    Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                    on: pg.skSudoOn
                                    onToggled: (v) => pg.sktoggle("sudo", v)
                                }
                            }

                            Item {
                                width: parent.width
                                height: Math.max(skPolkitCol.height, skPolkitSw.implicitHeight)
                                visible: pg.skSupported
                                Column {
                                    id: skPolkitCol
                                    anchors { left: parent.left; right: skPolkitSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                    spacing: 2
                                    Text { text: I18n.tr("Admin prompts"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                    Text {
                                        width: parent.width
                                        text: I18n.tr("Use your security key for graphical admin prompts")
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                        wrapMode: Text.WordWrap
                                    }
                                }
                                Sw {
                                    id: skPolkitSw
                                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                    opacity: pg.skPending === "toggle" && pg.skPendingTarget === "polkit" ? 0.4 : 1
                                    Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                    on: pg.skPolkitOn
                                    onToggled: (v) => pg.sktoggle("polkit", v)
                                }
                            }

                            Item {
                                width: parent.width
                                height: Math.max(skLoginCol.height, skLoginSw.implicitHeight)
                                visible: pg.skSupported
                                Column {
                                    id: skLoginCol
                                    anchors { left: parent.left; right: skLoginSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                    spacing: 2
                                    Text { text: I18n.tr("Sign-in screen"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                    Text {
                                        width: parent.width
                                        text: I18n.tr("Use your security key at the SDDM greeter")
                                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                        wrapMode: Text.WordWrap
                                    }
                                }
                                Sw {
                                    id: skLoginSw
                                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                    opacity: pg.skPending === "toggle" && pg.skPendingTarget === "login" ? 0.4 : 1
                                    Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                    on: pg.skLoginOn
                                    onToggled: (v) => pg.sktoggle("login", v)
                                }
                            }
                        }

                        Rectangle { width: parent.width; height: Tokens.border; color: Tokens.lineSoft }

                        Column {
                            width: parent.width
                            spacing: Tokens.s2
                            Text {
                                width: parent.width
                                text: I18n.tr("Security key behavior")
                                color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            }
                            Row { topPadding: Tokens.s1; spacing: Tokens.s2
                                Chip { label: I18n.tr("Security key or password"); mode: "sk-either"; kind: "security-key" }
                                Chip { label: I18n.tr("Security key + password"); mode: "sk-mfa"; kind: "security-key" }
                            }
                            Text {
                                width: parent.width
                                text: pg.skModeLine
                                color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                wrapMode: Text.WordWrap
                            }
                        }

                        Item {
                            width: parent.width
                            height: Math.max(skTouchCol.height, skTouchSw.implicitHeight)
                            visible: pg.skSupported
                            Column {
                                id: skTouchCol
                                anchors { left: parent.left; right: skTouchSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                spacing: 2
                                Text { text: I18n.tr("Touch requirement"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                Text {
                                    width: parent.width
                                    text: I18n.tr("Require touching the key during authentication")
                                    color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    wrapMode: Text.WordWrap
                                }
                            }
                            Sw {
                                id: skTouchSw
                                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                opacity: pg.skPending === "flag" && pg.skPendingTarget === "touch-required" ? 0.4 : 1
                                Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                on: pg.skTouchRequired
                                onToggled: (v) => pg.sksetFlag("touch-required", v)
                            }
                        }

                        Item {
                            width: parent.width
                            height: Math.max(skPinCol.height, skPinSw.implicitHeight)
                            visible: pg.skSupported
                            Column {
                                id: skPinCol
                                anchors { left: parent.left; right: skPinSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                spacing: 2
                                Text { text: I18n.tr("PIN verification"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                Text {
                                    width: parent.width
                                    text: I18n.tr("Require the authenticator PIN when the key supports it")
                                    color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    wrapMode: Text.WordWrap
                                }
                            }
                            Sw {
                                id: skPinSw
                                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                opacity: pg.skPending === "flag" && pg.skPendingTarget === "pin-verification" ? 0.4 : 1
                                Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                on: pg.skPinVerification
                                onToggled: (v) => pg.sksetFlag("pin-verification", v)
                            }
                        }

                        Item {
                            width: parent.width
                            height: Math.max(skUvCol.height, skUvSw.implicitHeight)
                            visible: pg.skSupported
                            Column {
                                id: skUvCol
                                anchors { left: parent.left; right: skUvSw.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                                spacing: 2
                                Text { text: I18n.tr("User verification"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                                Text {
                                    width: parent.width
                                    text: I18n.tr("Require the key's built-in user verification when available")
                                    color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    wrapMode: Text.WordWrap
                                }
                            }
                            Sw {
                                id: skUvSw
                                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                                opacity: pg.skPending === "flag" && pg.skPendingTarget === "user-verification" ? 0.4 : 1
                                Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                                on: pg.skUserVerification
                                onToggled: (v) => pg.sksetFlag("user-verification", v)
                            }
                        }

                        Text {
                            width: parent.width
                            visible: pg.skSupported
                            text: I18n.tr("Real key setup that needs a PIN works best from a terminal right now. The Hub can show status and policy, but the underlying enrollment tool still prompts on stdin.")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }

                        Text {
                            width: parent.width
                            visible: pg.skSupported
                            text: I18n.tr("Lock screen security-key unlock is not wired yet. This version handles enrollment plus sudo, admin prompts, and the sign-in screen.")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }
                }
                }
                }

                // vertical rule between the halves
                Rectangle {
                    width: Tokens.border
                    height: Math.max(settingsLeft.height, settingsRight.height)
                    color: Tokens.lineSoft
                }

                // ── right: the enrolled prints ──
                Column {
                    id: settingsRight
                    width: settCol.width - settingsLeft.width - Tokens.s5 * 2 - Tokens.border
                    spacing: Tokens.s2

                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        Text { text: I18n.tr("Prints"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.capitalization: Font.AllUppercase; font.letterSpacing: Tokens.trackMark }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.max(0, parent.width - x)
                            horizontalAlignment: Text.AlignRight
                            text: pg.fpending === "enroll" ? I18n.tr("ENROLLING\u2026") : (pg.floading ? I18n.tr("Checking\u2026") : "")
                            color: Tokens.inkFaint; font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny; elide: Text.ElideRight
                        }
                    }

                    // live scan while enrolling (recording) or verifying (using)
                    Item {
                        width: parent.width
                        height: visible ? 96 : 0
                        visible: pg.fpending === "enroll" || pg.fpending === "verify"
                        FingerprintScan {
                            anchors.centerIn: parent
                            sizePx: 84
                            accent: Tokens.sun
                            ink: Tokens.ink
                            phase: pg.fpending === "enroll" ? "enroll"
                                 : (pg.fpending === "verify" ? "scanning" : "ready")
                            progress: pg.fstagesTotal > 0 ? pg.fstagesPassed / pg.fstagesTotal : 0
                        }
                    }
                    Text {
                        width: parent.width
                        visible: !pg.floading && pg.ffingers.length === 0
                        text: I18n.tr("No prints yet. Enroll one below and it will show up here.")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                    }

                    // enrolled fingers: one quiet row each, delete behind an
                    // armed second click so nothing vanishes on a stray tap.
                    Column {
                        id: fingerList
                        width: parent.width
                        visible: pg.ffingers.length > 0
                        spacing: 0

                        Repeater {
                            model: pg.ffingers
                            delegate: Rectangle {
                                id: fingerRow
                                required property var modelData
                                required property int index
                                readonly property bool armDel: pg.fdelConfirmFor === modelData
                                width: fingerList.width
                                height: 34
                                color: delHover.hovered ? Tokens.tint5 : "transparent"
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                Text {
                                    id: fingerName
                                    anchors { left: parent.left; leftMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                                    width: Math.min(220, fingerRow.width - 230)
                                    text: pg.ffriendly(fingerRow.modelData)
                                    color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    elide: Text.ElideRight
                                }
                                Text {
                                    anchors { left: fingerName.right; leftMargin: Tokens.s3; right: fingerActions.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                    text: fingerRow.modelData
                                    color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                                    elide: Text.ElideRight
                                }
                                Row {
                                    id: fingerActions
                                    anchors { right: parent.right; rightMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                                    spacing: Tokens.s2
                                    Btn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("VERIFY"); compact: true
                                        armed: pg.fpending === "" && pg.fdaemon && pg.fready
                                        onAct: pg.fstartVerify(fingerRow.modelData)
                                    }
                                    Btn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: fingerRow.armDel ? I18n.tr("SURE?") : "\u2715"
                                        compact: true
                                        armed: pg.fpending === "" && pg.ffingers.length > 0
                                        onAct: {
                                            if (fingerRow.armDel) {
                                                confirmReset.stop();
                                                pg.fdelete(fingerRow.modelData);
                                            } else {
                                                pg.frmConfirm = false;
                                                pg.fdelConfirmFor = fingerRow.modelData;
                                                confirmReset.restart();
                                            }
                                        }
                                    }
                                }
                                HoverHandler { id: delHover; cursorShape: Qt.ArrowCursor }
                                // hairline between rows, never after the last
                                Rectangle {
                                    anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                                    height: Tokens.border; color: Tokens.lineSoft
                                    visible: fingerRow.index < pg.ffingers.length - 1
                                }
                            }
                        }
                    }

                    // actions
                    Column {
                        width: parent.width
                        topPadding: Tokens.s1
                        spacing: Tokens.s2

                        Btn {
                            width: parent.width
                            text: pg.fpending === "enroll" ? I18n.tr("ENROLLING\u2026") : I18n.tr("ENROLL FINGERPRINT")
                            primary: true
                            armed: pg.fpending === "" && pg.fdaemon && pg.fready
                            onAct: pg.fstartEnroll()
                        }
                        Row {
                            spacing: Tokens.s2
                            Btn {
                                text: pg.fpending === "verify" ? I18n.tr("VERIFYING\u2026") : I18n.tr("VERIFY")
                                compact: true
                                armed: pg.fpending === "" && pg.fdaemon && pg.fready && pg.ffingers.length > 0
                                onAct: pg.fstartVerify("")
                            }
                            Btn {
                                text: pg.frmConfirm ? I18n.tr("CONFIRM \u00b7 REMOVE ALL") : I18n.tr("REMOVE ALL")
                                compact: true
                                armed: pg.ffingers.length > 0 && pg.fpending === ""
                                onAct: {
                                    if (pg.frmConfirm) {
                                        confirmReset.stop();
                                        pg.fdelete("");
                                    } else {
                                        pg.fdelConfirmFor = "";
                                        pg.frmConfirm = true;
                                        confirmReset.restart();
                                    }
                                }
                            }
                        }
                    }

                    Text {
                        width: parent.width
                        visible: pg.ferr !== ""
                        topPadding: Tokens.s1
                        text: pg.ferr
                        color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; wrapMode: Text.WordWrap
                    }

                    Rectangle { width: parent.width; height: Tokens.border; color: Tokens.line }

                    Column {
                        width: parent.width
                        spacing: Tokens.s2

                        Row {
                            width: parent.width
                            spacing: Tokens.s2
                            Text { text: I18n.tr("Passkeys"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.capitalization: Font.AllUppercase; font.letterSpacing: Tokens.trackMark }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                width: Math.max(0, parent.width - x)
                                horizontalAlignment: Text.AlignRight
                                text: pg.skPending === "enroll" ? I18n.tr("SETTING UP\u2026") : pg.skStatusLine
                                color: Tokens.inkFaint; font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny; elide: Text.ElideRight
                            }
                        }

                        Text {
                            width: parent.width
                            visible: !pg.skLoading && !pg.skSupported
                            text: I18n.tr("Install pam-u2f to enroll a FIDO2 or U2F security key here.")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }

                        Text {
                            width: parent.width
                            visible: !pg.skLoading && pg.skSupported && !pg.skDevicePresent
                            text: I18n.tr("Insert your YubiKey or other security key, then retry setup.")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }

                        Text {
                            width: parent.width
                            visible: !pg.skLoading && pg.skSupported && pg.skDevicePresent && !pg.skEnrolled
                            text: I18n.tr("No passkeys yet. Set up the inserted security key below and it will show up here.")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }

                        Text {
                            width: parent.width
                            visible: !pg.skLoading && pg.skSupported && pg.skDevicePresent && pg.skEnrolled
                            text: I18n.tr("Use Add security key to enroll another key, or remove an old one below.")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }

                        Btn {
                            width: parent.width
                            visible: !pg.skLoading && pg.skSupported && !pg.skDevicePresent
                            text: I18n.tr("RETRY SECURITY KEY")
                            compact: true
                            armed: pg.skPending === ""
                            onAct: pg.skreload()
                        }

                        Column {
                            width: parent.width
                            visible: pg.skEnrolled
                            spacing: 0

                            Repeater {
                                model: pg.skCredentialIds
                                delegate: Rectangle {
                                    id: passkeyRow
                                    required property var modelData
                                    required property int index
                                    width: parent.width
                                    height: 34
                                    color: "transparent"

                                    Text {
                                        id: passkeyName
                                        anchors { left: parent.left; leftMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                                        width: Math.min(180, passkeyRow.width - 170)
                                        text: passkeyRow.modelData.label || I18n.tr("Security key %1").arg(passkeyRow.modelData.id)
                                        color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                        elide: Text.ElideRight
                                    }
                                    Text {
                                        anchors { left: passkeyName.right; leftMargin: Tokens.s3; right: skRemoveBtn.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                        text: I18n.tr("Credential %1").arg(passkeyRow.modelData.id)
                                        color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                                        elide: Text.ElideRight
                                    }
                                    Btn {
                                        id: skRemoveBtn
                                        anchors { right: parent.right; rightMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                                        text: pg.skPending === "remove" && pg.skPendingTarget === passkeyRow.modelData.id ? I18n.tr("REMOVING\u2026") : I18n.tr("REMOVE")
                                        compact: true
                                        armed: pg.skPending === ""
                                        onAct: pg.skremove(passkeyRow.modelData.id)
                                    }
                                    Rectangle {
                                        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                                        height: Tokens.border; color: Tokens.lineSoft
                                        visible: passkeyRow.index < pg.skCredentialIds.length - 1
                                    }
                                }
                            }
                        }

                        Btn {
                            width: parent.width
                            text: pg.skPending === "enroll" ? I18n.tr("SETTING UP\u2026") : (pg.skEnrolled ? I18n.tr("ADD SECURITY KEY") : I18n.tr("SET UP SECURITY KEY"))
                            primary: true
                            armed: pg.skPending === "" && pg.skSupported && pg.skDevicePresent
                            onAct: pg.skenroll()
                        }

                        Text {
                            width: parent.width
                            visible: pg.skError !== ""
                            text: pg.skError
                            color: Tokens.ink; font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; wrapMode: Text.WordWrap
                        }
                    }
                }
            }
        }
    }

    // ── the gallery grid ────────────────────────────────────────────────────
    Flickable {
        id: flick
        anchors {
            left: parent.left; right: parent.right
            top: sett.bottom; bottom: parent.bottom
            leftMargin: Tokens.s6; rightMargin: Tokens.s6
            topMargin: Tokens.s4; bottomMargin: Tokens.s6
        }
        visible: !pg.loading && !pg.loadFailed && pg.shown.length > 0
        opacity: visible ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
        contentWidth: width
        contentHeight: masonry.implicitHeight + Tokens.s4
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Row {
            id: masonry
            // the masonry fills the body it is given; the cards keep the measure
            width: flick.width - Tokens.s3
            x: Math.round((flick.width - width) / 2)
            spacing: Tokens.s3

            Repeater {
                model: pg.cols
                delegate: Column {
                    id: column
                    required property int index
                    width: (masonry.width - (pg.cols - 1) * Tokens.s3) / pg.cols
                    spacing: Tokens.s3

                    Repeater {
                        model: pg.grouped[column.index] || []
                        delegate: LockTile {
                            required property var modelData
                            width: column.width
                            viewport: flick
                            skin: modelData
                            ordinal: modelData.ordinal || 0
                            active: modelData.slug === pg.active
                            busy: pg.pendingSlug === modelData.slug
                            onApplied: pg.select(modelData)
                            onPreviewed: pg.preview(modelData.slug)
                        }
                    }
                }
            }
        }
    }

    // ── a stroked vector glyph (viewBox 24), tint-able ──────────────────────
    component Glyph: Item {
        id: g
        property string path: ""
        property color tint: Tokens.inkMuted
        property real size: 20
        property real weight: 1.7
        implicitWidth: size
        implicitHeight: size
        Shape {
            anchors.centerIn: parent
            width: 24; height: 24
            scale: g.size / 24
            preferredRendererType: Shape.CurveRenderer
            antialiasing: true
            ShapePath {
                strokeColor: g.tint
                strokeWidth: g.weight
                fillColor: "transparent"
                capStyle: ShapePath.RoundCap
                joinStyle: ShapePath.RoundJoin
                PathSvg { path: g.path }
            }
        }
    }

    // ── one sign-in mode chip ───────────────────────────────────────────────
    // same grammar as a tile: selected = ink border + tint10 fill + a dot.
    component Chip: Rectangle {
        id: chip
        property string label: ""
        property string mode: ""
        property string kind: "keyring"
        readonly property bool on: chip.kind === "security-key"
            ? ((chip.mode === "sk-mfa" && pg.skAuthMode === "mfa") || (chip.mode === "sk-either" && pg.skAuthMode !== "mfa"))
            : (pg.kmode === chip.mode)
        readonly property bool busy: chip.kind === "security-key"
            ? (pg.skPending === "mode" && ((chip.mode === "sk-mfa" && pg.skPendingTarget === "mfa") || (chip.mode === "sk-either" && pg.skPendingTarget === "either")))
            : (pg.kpending === chip.mode)
        implicitWidth: chLab.implicitWidth + Tokens.s4 * 2
        height: Tokens.ctlH + 4
        radius: Tokens.radius
        color: chip.on ? Tokens.tint10 : (chHover.hovered ? Tokens.tint5 : "transparent")
        border.width: Tokens.border
        border.color: chip.on ? Tokens.ink : (chHover.hovered ? Tokens.lineStrong : Tokens.line)
        opacity: (((chip.kind === "security-key") ? (pg.skPending !== "") : (pg.kpending !== "")) && !chip.busy) ? 0.4 : 1
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
        Behavior on opacity { NumberAnimation { duration: Tokens.snap } }

        Row {
            id: chLab
            anchors.centerIn: parent
            spacing: Tokens.s1
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                visible: chip.on
                width: 5; height: 5; radius: 2.5; color: Tokens.ink
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: chip.busy ? (I18n.tr(chip.label) + "\u2026") : I18n.tr(chip.label)
                color: chip.on ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                font.weight: chip.on ? Font.DemiBold : Font.Medium
            }
        }

        HoverHandler { id: chHover; cursorShape: Qt.PointingHandCursor }
        TapHandler {
            onTapped: {
                if (chip.kind === "security-key")
                    pg.sksetMode(chip.mode === "sk-mfa" ? "mfa" : "either");
                else
                    pg.kchoose(chip.mode);
            }
        }
    }

    // ── one lock-skin tile ──────────────────────────────────────────────────
    // gallery grammar: selected = ink border + tint10 fill + corner dot. The
    // hero is the real skin thumbnail (a permitted colour specimen) when the
    // theme shipped a preview.gif, else a drawn lock silhouette.
    component LockTile: Rectangle {
        id: tile

        property var skin: ({})
        property int ordinal: 0
        property bool active: false
        property bool busy: false      // an apply is in flight for this skin
        property Flickable viewport: null
        signal applied()
        signal previewed()

        // near-viewport test: map the tile into the Flickable and keep a 600px
        // margin so a remote thumbnail loads just before it scrolls in. reading
        // contentY/height makes the binding re-eval as the list scrolls.
        readonly property bool onScreen: {
            if (!viewport)
                return true;
            viewport.contentY;
            viewport.height;
            var top = tile.mapToItem(viewport, 0, 0).y;
            return top < viewport.height + 600 && top + tile.height > -600;
        }

        implicitHeight: body.implicitHeight + Tokens.s6
        radius: Tokens.radius
        color: tile.active ? Tokens.tint10 : (hover.hovered ? Tokens.tint5 : "transparent")
        border.width: Tokens.border
        border.color: tile.active ? Tokens.ink : (hover.hovered ? Tokens.lineStrong : Tokens.line)
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

        // gallery selection dot
        Rectangle {
            anchors { top: parent.top; right: parent.right; topMargin: Tokens.s2; rightMargin: Tokens.s2 }
            width: 5; height: 5; radius: 2.5
            color: Tokens.ink
            visible: tile.active
        }

        Column {
            id: body
            anchors { left: parent.left; right: parent.right; top: parent.top }
            anchors.margins: Tokens.s4
            spacing: 0

            // preview hero: 16:9 well over paper, hairline frame.
            Rectangle {
                id: media
                width: parent.width
                height: Math.round(width * 9 / 16)
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: (tile.active || hover.hovered) ? Tokens.ink : Tokens.line
                clip: true
                Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                // the real skin, animated. a lock thumbnail is a permitted
                // colour specimen, so it stays as shipped.
                AnimatedImage {
                    id: gif
                    anchors.fill: parent
                    anchors.margins: 1
                    source: tile.onScreen ? (tile.skin.preview || "") : ""
                    fillMode: Image.PreserveAspectCrop
                    cache: false
                    asynchronous: true
                    playing: tile.onScreen
                }

                // silhouette: shown while the thumbnail loads, or when the skin
                // shipped none.
                Glyph {
                    anchors.centerIn: parent
                    visible: gif.status !== AnimatedImage.Ready
                    path: pg.pLock; size: 34; tint: Tokens.inkMuted
                }

                // Installed themes can always be previewed with the real lock.
                Rectangle {
                    anchors { left: parent.left; bottom: parent.bottom; margins: Tokens.s2 }
                    width: pvRow.implicitWidth + Tokens.s4
                    height: Tokens.ctlH
                    radius: Tokens.radius
                    visible: !tile.busy
                    // a scrim over a colour specimen to seat the label, not an
                    // app-surface fill.
                    color: Qt.rgba(0, 0, 0, 0.55)
                    border.width: Tokens.border
                    border.color: pvArea.containsMouse ? Tokens.ink : Tokens.lineStrong
                    opacity: (hover.hovered || pvArea.containsMouse) ? 1 : 0
                    Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                    Row {
                        id: pvRow
                        anchors.centerIn: parent
                        spacing: Tokens.s1
                        Glyph {
                            anchors.verticalCenter: parent.verticalCenter
                            path: pg.pPlay; size: 11; weight: 2; tint: Tokens.ink
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("Preview"); color: Tokens.ink
                            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium
                        }
                    }
                    MouseArea {
                        id: pvArea
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: tile.previewed()
                    }
                }

                // apply overlay
                Rectangle {
                    anchors.fill: parent
                    visible: tile.busy
                    color: Qt.rgba(0, 0, 0, 0.6)
                    Column {
                        anchors.centerIn: parent
                        spacing: Tokens.s2
                        Glyph {
                            anchors.horizontalCenter: parent.horizontalCenter
                            path: pg.pRefresh; size: 22; weight: 2; tint: Tokens.ink
                            RotationAnimator on rotation {
                                from: 0; to: 360; duration: 900; loops: Animation.Infinite; running: tile.busy
                            }
                        }
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: I18n.tr("Applying\u2026"); color: Tokens.ink
                            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
                        }
                    }
                }
            }

            Item { width: 1; height: Tokens.s4 }

            // ordinal + state badge
            Item {
                width: parent.width
                height: number.implicitHeight

                Text {
                    id: number
                    anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                    // an index is file-truth chrome, so mono.
                    text: (tile.ordinal < 10 ? "0" : "") + tile.ordinal
                    color: (tile.active || hover.hovered) ? Tokens.ink : Tokens.inkFaint
                    font.family: Tokens.mono; font.pixelSize: Tokens.fValue
                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                }

                Row {
                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                    spacing: Tokens.s1 + 3
                    visible: tile.busy || tile.active
                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: 6; height: 6; radius: 3; color: Tokens.ink
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: tile.busy ? I18n.tr("APPLYING") : I18n.tr("ACTIVE")
                        color: Tokens.ink
                        font.family: Tokens.ui; font.pixelSize: 10
                        font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                    }
                }
            }

            // theme family tags
            Text {
                width: parent.width
                topPadding: Tokens.s3
                visible: (tile.skin.tags || []).length > 0
                text: (tile.skin.tags || []).join("  \u00b7  ")
                color: Tokens.inkFaint
                font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                font.capitalization: Font.AllUppercase
                elide: Text.ElideRight
            }

            // skin name
            Text {
                width: parent.width
                topPadding: (tile.skin.tags || []).length > 0 ? Tokens.s2 : Tokens.s3
                text: tile.skin.name || ""
                color: Tokens.ink
                font.family: Tokens.ui; font.pixelSize: Tokens.fRow; font.weight: Font.DemiBold
                elide: Text.ElideRight
            }

            // one-line summary
            Text {
                width: parent.width
                topPadding: Tokens.s1
                visible: (tile.skin.summary || "") !== ""
                text: tile.skin.summary || ""
                color: Tokens.inkDim
                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                elide: Text.ElideRight
            }

            // blurb, two lines
            Text {
                width: parent.width
                topPadding: Tokens.s2
                visible: (tile.skin.blurb || "") !== ""
                text: tile.skin.blurb || ""
                color: Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: 12
                lineHeight: 1.32
                wrapMode: Text.WordWrap
                maximumLineCount: 2
                elide: Text.ElideRight
            }
        }

        // hover affordance: this tile will apply on click.
        Glyph {
            anchors { right: parent.right; bottom: parent.bottom; rightMargin: Tokens.s4; bottomMargin: Tokens.s4 }
            path: pg.pChevron; size: 15; weight: 2; rotation: -90
            tint: Tokens.ink
            opacity: (hover.hovered && !tile.active && !tile.busy) ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
        }

        HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
        TapHandler { onTapped: if (!tile.active && !tile.busy) tile.applied() }
    }

    component FpRing: Item {
        id: ring
        property real frac: 0            // 0..1 once the total is known
        property bool spin: false        // indeterminate until num-stages lands
        implicitWidth: 108
        implicitHeight: 108

        // track
        Shape {
            anchors.fill: parent
            preferredRendererType: Shape.CurveRenderer
            antialiasing: true
            ShapePath {
                strokeColor: Tokens.lineSoft
                strokeWidth: 3
                fillColor: "transparent"
                capStyle: ShapePath.RoundCap
                PathAngleArc { centerX: 54; centerY: 54; radiusX: 48; radiusY: 48; startAngle: 90; sweepAngle: 360 }
            }
        }
        // progress; hidden while spinning so the sweep never lies
        Shape {
            anchors.fill: parent
            visible: !ring.spin
            preferredRendererType: Shape.CurveRenderer
            antialiasing: true
            ShapePath {
                strokeColor: Tokens.ink
                strokeWidth: 3
                fillColor: "transparent"
                capStyle: ShapePath.RoundCap
                PathAngleArc {
                    centerX: 54; centerY: 54; radiusX: 48; radiusY: 48; startAngle: 90
                    sweepAngle: Math.max(2, ring.frac * 358)
                    Behavior on sweepAngle { NumberAnimation { duration: 380; easing.type: Easing.OutCubic } }
                }
            }
        }
        RotationAnimator on rotation {
            from: 0; to: 360; duration: 1100
            loops: Animation.Infinite; running: ring.spin
        }
    }

    // modal view-state helpers, kept here so bindings below stay readable.
    readonly property bool fbusy: pg.fpending === "enroll" || pg.fpending === "verify"
    readonly property bool fmatchWin: pg.fstatus === "verify-match"
    readonly property real fringFrac: pg.fstagesTotal > 0 ? Math.min(1, pg.fstagesPassed / pg.fstagesTotal) : 0

    MouseArea {
        anchors.fill: parent
        enabled: pg.foverlay
        visible: pg.foverlay
        z: 998
        // an outside click only dismisses when nothing is running: mid-enroll
        // it would silently throw away every captured stage.
        onClicked: { if (!pg.fbusy && !pg.fnaming) pg.fcloseOverlay() }
    }

    Rectangle {
        id: fplate
        anchors.centerIn: parent
        visible: pg.foverlay
        z: 999
        width: Math.min(parent.width - Tokens.s6 * 2, 420)
        height: 368
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        focus: true
        Keys.onEscapePressed: pg.fcloseOverlay()

        Column {
            id: fbodyCol
            anchors.fill: parent
            anchors.margins: Tokens.s5
            spacing: Tokens.s4

            // header: eyebrow + close
            Item {
                width: parent.width
                height: 16
                Text {
                    anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                    text: pg.fnaming ? I18n.tr("NAME THIS PRINT")
                        : (pg.foverlayMode === "enroll" ? I18n.tr("ENROLLMENT") : I18n.tr("VERIFY"))
                    color: Tokens.inkFaint; font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark
                }
                // close: a real hit area, not a bare glyph
                Item {
                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                    width: 28; height: 28
                    Glyph {
                        anchors.centerIn: parent
                        path: pg.pX; size: 13; weight: 2
                        tint: fpCloseMa.hovered ? Tokens.ink : Tokens.inkMuted
                        Behavior on tint { ColorAnimation { duration: Tokens.snap } }
                    }
                    HoverHandler { id: fpCloseMa; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: pg.fcloseOverlay() }
                }
            }

            // body: ring + readout + guidance, or the naming step
            Item {
                width: parent.width
                height: fplate.height - Tokens.s5 * 2 - 16 - logCol.height - footRow.height - Tokens.s4 * 3

                // live enroll: ring fills stage by stage
                Column {
                    anchors.centerIn: parent
                    visible: pg.foverlayMode === "enroll" && !pg.fnaming
                    spacing: Tokens.s3

                    Item {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: 108; height: 108
                        FpRing {
                            anchors.fill: parent
                            visible: pg.fstatus !== "enroll-completed"
                            frac: pg.fringFrac
                            spin: pg.fstagesTotal === 0 && pg.fpending === "enroll"
                        }
                        Glyph {
                            anchors.centerIn: parent
                            visible: pg.fstatus === "enroll-completed"
                            path: pg.pCheck; size: 44; weight: 2.2; tint: Tokens.ink
                        }
                        Text {
                            anchors.centerIn: parent
                            visible: pg.fstatus !== "enroll-completed" && pg.fstagesTotal > 0
                            text: pg.fstagesPassed + "/" + pg.fstagesTotal
                            color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fValue
                        }
                    }
                    Text {
                        width: 300
                        horizontalAlignment: Text.AlignHCenter
                        wrapMode: Text.WordWrap
                        text: pg.fheadline("enroll", pg.fstatus)
                        color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow; font.weight: Font.DemiBold
                    }
                    Text {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        text: pg.fstatus === "" ? I18n.tr("Each pass fills the ring \u2014 keep touching until it closes")
                                                : (pg.fdeviceName !== "" ? pg.fdeviceName : "")
                        color: Tokens.inkDim; font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        elide: Text.ElideRight
                    }
                }

                // verify: spin while scanning, then the verdict
                Column {
                    anchors.centerIn: parent
                    visible: pg.foverlayMode === "verify"
                    spacing: Tokens.s3

                    Item {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: 108; height: 108
                        FpRing {
                            anchors.fill: parent
                            visible: pg.fpending === "verify"
                            frac: 0
                            spin: true
                        }
                        Glyph {
                            anchors.centerIn: parent
                            visible: pg.fpending !== "verify" && pg.fmatchWin
                            path: pg.pCheck; size: 44; weight: 2.2; tint: Tokens.ink
                        }
                        Glyph {
                            anchors.centerIn: parent
                            visible: pg.fpending !== "verify" && !pg.fmatchWin
                            path: pg.pX; size: 40; weight: 2.2; tint: Tokens.ink
                        }
                    }
                    Text {
                        width: 300
                        horizontalAlignment: Text.AlignHCenter
                        wrapMode: Text.WordWrap
                        text: pg.fheadline("verify", pg.fstatus)
                        color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow; font.weight: Font.DemiBold
                    }
                    Text {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        visible: pg.fpending === "verify" && pg.ffingers.length > 0
                        text: pg.ffingers.length === 1 ? I18n.tr("Comparing against one enrolled finger") : I18n.tr("Comparing against %1 enrolled fingers").arg(pg.ffingers.length)
                        color: Tokens.inkDim; font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                }

                // naming step: the print exists, give it a label
                Column {
                    anchors.centerIn: parent
                    visible: pg.fnaming
                    spacing: Tokens.s2

                    Text {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        text: I18n.tr("Enrolled. Name it so the list stays readable.")
                        color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }
                    Field {
                        id: nameField
                        width: parent.width
                        text: pg.fnameDraft
                        placeholder: I18n.tr("e.g. right index")
                        onEdited: (v) => pg.fnameDraft = v
                        onAccepted: pg.fsaveName()
                    }
                    Connections {
                        target: pg
                        function onFnamingChanged() {
                            if (pg.fnaming)
                                nameField.grabFocus()
                        }
                    }
                }
            }

            // log strip: the raw conversation, demoted but not gone
            Column {
                id: logCol
                width: parent.width
                spacing: 3
                Rectangle { width: parent.width; height: Tokens.border; color: Tokens.lineSoft }
                Text {
                    width: parent.width
                    text: pg.ftail[0]
                    color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: pg.ftail[1] || " "
                    color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                    elide: Text.ElideRight
                }
            }

            // footer: state word left, actions right. Every mode of the modal
            // resolves from this one row: abort a running scan, save/skip a
            // fresh print, close a finished one.
            Item {
                id: footRow
                width: parent.width
                height: 32
                Row {
                    anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                    spacing: Tokens.s2
                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: 6; height: 6; radius: 3
                        color: Tokens.ink
                        visible: pg.fbusy
                        SequentialAnimation on opacity {
                            loops: Animation.Infinite; running: pg.fbusy
                            NumberAnimation { from: 1; to: 0.25; duration: 600 }
                            NumberAnimation { from: 0.25; to: 1; duration: 600 }
                        }
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(240, fplate.width - 170)
                        text: pg.fbusy ? (pg.fpending === "enroll" ? I18n.tr("Enrolling") : I18n.tr("Verifying")) : pg.fresult
                        color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                        elide: Text.ElideRight
                    }
                }
                Row {
                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                    spacing: Tokens.s2
                    Btn {
                        visible: pg.fnaming
                        text: I18n.tr("SKIP"); compact: true
                        armed: pg.fnameDraft.trim() === ""
                        onAct: pg.fskipName()
                    }
                    Btn {
                        visible: pg.fnaming
                        text: I18n.tr("SAVE"); compact: true
                        primary: true
                        armed: pg.fnameDraft.trim() !== ""
                        onAct: pg.fsaveName()
                    }
                    Btn {
                        visible: !pg.fnaming
                        text: pg.fbusy ? I18n.tr("ABORT") : I18n.tr("CLOSE")
                        compact: true
                        onAct: pg.fcloseOverlay()
                    }
                }
            }
        }

        Connections {
            target: pg
            function onFoverlayChanged() {
                if (pg.foverlay)
                    fplate.forceActiveFocus()
            }
        }
    }
}
