// SddmShim.qml - Quickshell shim for Ryoku in-session lock
//
// Exposes the same API as a real SDDM greeter (login, reboot, powerOff,
// suspend, userModel, sessionModel, keyboard) so themes work unchanged,
// plus fingerprint state properties the theme reads for the sensor hint.
//
// Auth flow — TWO parallel PAM conversations (pam_fprintd's man page is
// explicit: PAM stacks are serialised by design, so parallel methods need
// separate conversations — "the way multiple authentication methods are
// made available to users of gdm"):
//   1. lock_shell.qml sets armWhenReady = true once WlSessionLock.secure
//   2. The shim probes fprintd-list + ~/.config/qylock/fingerprint
//   3. Stale fprintd-verify processes are cleared, then BOTH start:
//        pamFp -> ryoku-lock     (fingerprint; timeout=-1 scans until match)
//        pamPw -> ryoku-lock-pw  (typed password; prompt pending from t0)
//   4. First conversation to succeed unlocks and aborts the other
//   5. A failed scan (3 misreads) re-arms after 1s; a wrong password
//      re-prompts immediately — neither method starves the other

import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.Pam

Item {
    id: shim

    // ── theme configuration ────────────────────────────────────────────────
    property string themePath: ""
    property var config: ({})
    property bool configReady: false
    property string hostName: "localhost"

    // ── fingerprint unlock state ────────────────────────────────────────────
    // These properties are read at lock start and updated live. The theme
    // binds to sddm.fingerprintHint, sddm.fingerprintReady,
    // sddm.fingerprintState, and sddm.fingerprintUnlock to show the sensor
    // hint and play the reveal flourish.
    property bool fpEnabled: true            // from ~/.config/qylock/fingerprint
    property bool fpHasFingers: false        // fprintd-list reports >= 1 finger
    readonly property bool fingerprintReady: fpEnabled && fpHasFingers
    property string fingerprintState: "idle" // idle | scanning | success | fail | unavailable
    property bool fingerprintUnlock: false   // true if sensor won (not typed)
    property bool armWhenReady: false        // lock surface is secured, arm now
    property bool armPending: false          // an armPrep run is in flight
    property bool fpTyped: false             // a key was fed to the conversation
    property bool unlocked: false            // a conversation already unlocked; guards double-fire

    // Held-sensor backoff (#243). A scan that dies almost as soon as it started
    // cannot be a misread — three real touches take seconds — it is fprintd
    // refusing the Claim (a ghost claim after suspend/resume). Timing the
    // failure is the only signal available without root, so it is the one used:
    // instant failures accumulate toward an "unavailable" state that stops the
    // per-second red loop and lets a backing-off probe wait for recovery.
    property real fpArmTs: 0                 // ms epoch the last scan was armed
    property int fpFastFails: 0              // consecutive instant (claim-held) failures
    property bool fpUnavailable: false       // sensor paused: held/busy, password still works
    property int fpBackoffMs: 5000           // re-probe gap while unavailable, doubling to a cap
    readonly property real fpInstantFailMs: 1500  // a failure faster than this cannot be a touch

    // arm the moment the lock secures; probes alone race and lose it.
    onArmWhenReadyChanged: {
        if (shim.armWhenReady)
            shim.maybeArm();
    }

    // orphaned verifiers hold the sensor claim; clear them before arming.
    Process {
        id: armPrepProc
        command: ["bash", "-c", "pkill -u \"$USER\" -x fprintd-verify 2>/dev/null; sleep 0.2"]
        onExited: () => {
            if (!shim.armPending)
                return;
            shim.armPending = false;
            shim.armFingerprintNow();
        }
    }

    // ── theme config loader ─────────────────────────────────────────────────
    // Parses theme.conf (key=value format) from the theme directory.
    function loadConfig(path) {
        if (!path) {
            config = { background: "bg.png" };
            configReady = true;
            return;
        }
        var url = "file://" + path + "/theme.conf";
        var xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState === XMLHttpRequest.DONE) {
                var newConfig = {};
                if ((xhr.status === 200 || xhr.status === 0) && xhr.responseText) {
                    var lines = xhr.responseText.split("\n");
                    for (var i = 0; i < lines.length; i++) {
                        var line = lines[i].trim();
                        if (line.startsWith("[") || line === "" || line.startsWith("#")) continue;
                        var parts = line.split("=");
                        if (parts.length === 2) {
                            newConfig[parts[0].trim()] = parts[1].trim();
                        }
                    }
                }
                if (!newConfig.background) {
                    newConfig.background = "bg.png";
                }
                config = newConfig;
                configReady = true;
            }
        };
        try {
            xhr.open("GET", url, true);
            xhr.send();
        } catch (e) {
            console.warn("SddmShim: failed to load theme.conf:", e);
            config = { background: "bg.png" };
            configReady = true;
        }
    }

    // ── user model ──────────────────────────────────────────────────────────
    // Single-user model for the lock screen. The real SDDM greeter enumerates
    // all system users; the in-session lock only needs the current user.
    property var userModel: ListModel {
        id: internalUserModel
        property string lastUser: Quickshell.env("USER") || "traveler"
        property int lastIndex: 0
        function rowCount() { return count; }
        function index(row, col) { return row; }
        function data(row, role) {
            var item = get(row);
            if (!item) return "";
            if (role === (Qt.UserRole + 1)) return item.name;
            if (role === (Qt.UserRole + 2)) return item.realName;
            return item.name;
        }
        Component.onCompleted: {
            append({
                name: Quickshell.env("USER") || "traveler",
                realName: Quickshell.env("USER") || "Traveler",
                icon: "",
                homeDir: "/home/" + (Quickshell.env("USER") || "traveler")
            })
        }
    }

    // ── session model ───────────────────────────────────────────────────────
    // Enumerates available desktop sessions from /usr/share/*-sessions/.
    property var sessionModel: ListModel {
        id: internalSessionModel
        property int lastIndex: 0
        // Running compositor's session id from the environment; the theme
        // prefers the installed session that matches this, by data not by name.
        property string desktopName: (Quickshell.env("XDG_SESSION_DESKTOP")
            || Quickshell.env("DESKTOP_SESSION")
            || Quickshell.env("XDG_CURRENT_DESKTOP") || "").split(":")[0].toLowerCase()
        function rowCount() { return count; }
        function index(row, col) { return row; }
        function data(row, role) {
            var item = get(row);
            if (!item) return "";
            return item.name;
        }
        Component.onCompleted: {
            append({ name: "Session", file: "" });
        }
    }

    Process {
        id: sessionEnumerator
        command: [
            "bash", "-c",
            "for f in /usr/share/wayland-sessions/*.desktop /usr/share/xsessions/*.desktop; do " +
            "if [ -f \"$f\" ]; then " +
            "NAME=$(grep -m1 '^Name=' \"$f\" | cut -d'=' -f2); " +
            "FILE=$(basename \"$f\"); " +
            "echo \"$NAME|||$FILE\"; " +
            "fi; done"
        ]
        stdout: StdioCollector {
            onStreamFinished: { shim.parseSessions(this.text); }
        }
        onExited: (exitCode, exitStatus) => {
            if (internalSessionModel.count === 0) {
                internalSessionModel.append({ name: "Unknown", file: "unknown.desktop" });
            }
        }
    }

    function parseSessions(output) {
        if (!output || output.trim() === "") {
            internalSessionModel.clear();
            internalSessionModel.append({ name: "Session", file: "unknown.desktop" });
            return;
        }
        internalSessionModel.clear();
        var lines = output.trim().split("\n");
        var currentDesktop = (Quickshell.env("XDG_SESSION_DESKTOP") || Quickshell.env("DESKTOP_SESSION") || "").toLowerCase();
        var bestIndex = 0;
        var added = 0;
        for (var i = 0; i < lines.length; i++) {
            var line = lines[i].trim();
            if (line === "") continue;
            var parts = line.split("|||");
            if (parts.length === 2 && parts[0] !== "" && parts[1] !== "") {
                internalSessionModel.append({ name: parts[0], file: parts[1] });
                var fileName = parts[1].toLowerCase();
                if (currentDesktop !== "" && (fileName.indexOf(currentDesktop) !== -1 || currentDesktop.indexOf(fileName.replace(".desktop", "")) !== -1)) {
                    bestIndex = added;
                }
                added++;
            }
        }
        if (added === 0) {
            internalSessionModel.append({ name: "Unknown", file: "unknown.desktop" });
        } else {
            internalSessionModel.lastIndex = bestIndex;
        }
    }

    // ── SDDM-compatible interface ───────────────────────────────────────────
    // Themes bind to sddm.login(), sddm.hostName, sddm.loginSucceeded,
    // sddm.loginFailed, etc. Under a real SDDM greeter these are provided by
    // the greeter; under the lock screen this shim provides them.
    property var sddm: QtObject {
        property string hostName: shim.hostName
        signal loginFailed()
        signal loginSucceeded()
        // lock_shell emits this once the secure surface is shown
        signal surfaceRevealed()

        // Fingerprint properties exposed to the theme. Under a real SDDM
        // greeter these are undefined, so theme gates on them evaluate false
        // and the greeter is visually untouched. Under this shim they drive
        // the sensor hint.
        readonly property bool fingerprintHint: true
        property bool fingerprintReady: shim.fingerprintReady
        property string fingerprintState: shim.fingerprintState
        property bool fingerprintUnlock: shim.fingerprintUnlock

        // login() is called by the theme when the user submits a password.
        // The key always goes to the dedicated password conversation
        // (pamPw): respond at once if its prompt is up, stash it if the
        // prompt has not arrived yet (onResponseRequiredChanged feeds it),
        // or start the conversation if it died.
        function login(user, password, sessionIndex) {
            pamPw.user = user;
            if (pamPw.active) {
                if (pamPw.responseRequired) {
                    shim.fpTyped = true;
                    pamPw.respond(password);
                    pamPw.pendingPassword = "";
                } else {
                    pamPw.pendingPassword = password;
                }
                return;
            }
            shim.fpTyped = false;
            pamPw.pendingPassword = password;
            if (password === "" && !shim.fingerprintReady)
                return;
            pamPw.start();
        }

        function reboot() { Quickshell.execDetached(["bash", "-c", "if [ -d /run/systemd/system ]; then systemctl reboot; else loginctl reboot; fi"]); }
        function powerOff() { Quickshell.execDetached(["bash", "-c", "if [ -d /run/systemd/system ]; then systemctl poweroff; else loginctl poweroff; fi"]); }
        function suspend() { Quickshell.execDetached(["bash", "-c", "if [ -d /run/systemd/system ]; then systemctl suspend; else loginctl suspend; fi"]); }
    }

    // SDDM exposes a writable `keyboard` carrying the lock-key state; skins
    // set `keyboard.numLock = true` so the numpad works. Provide it so the
    // assignment resolves instead of raising a ReferenceError.
    property var keyboard: QtObject {
        property bool numLock: false
        property bool capsLock: false
    }

    // ── hostname resolver ───────────────────────────────────────────────────
    // Resolve the real hostname for sddm.hostName; the "localhost" default
    // above keeps the isQuickshell test correct until this returns.
    Process {
        id: hostnameProc
        command: ["cat", "/etc/hostname"]
        stdout: StdioCollector {
            onStreamFinished: {
                var h = this.text.trim();
                if (h !== "") shim.hostName = h;
            }
        }
    }

    // ── fingerprint readiness probes ────────────────────────────────────────
    // Two independent probes determine if the sensor is available:
    //   1. fpToggleProc reads ~/.config/qylock/fingerprint (the Settings toggle)
    //   2. fpListProc runs fprintd-list to check for enrolled fingers
    // Both call maybeArm() when done, which starts the PAM conversation if
    // all conditions are met.

    // Read the Settings toggle (missing file = enabled).
    Process {
        id: fpToggleProc
        command: [
            "bash", "-c",
            "if [ -f \"$HOME/.config/qylock/fingerprint\" ]; then cat \"$HOME/.config/qylock/fingerprint\"; else printf 'on'; fi"
        ]
        stdout: StdioCollector {
            onStreamFinished: {
                var v = this.text.trim().toLowerCase();
                shim.fpEnabled = !(v === "off" || v === "0" || v === "false");
                shim.maybeArm();
            }
        }
    }

    // Probe enrolled fingers; the lock only offers the sensor when this user
    // actually has one stored. fprintd itself is D-Bus activated, so right at
    // lock time the daemon (and the device) may not be up yet -- a failed or
       // finger-less probe is retried a few times while the lock is held.
    Process {
        id: fpListProc
        command: ["fprintd-list", Quickshell.env("USER") || "traveler"]
        stdout: StdioCollector {
            onStreamFinished: {
                var txt = this.text || "";
                var hasDev = txt.indexOf("Using device") !== -1 || txt.indexOf("Device at") !== -1 || txt.indexOf("found ") !== -1;
                var n = 0;
                var lines = txt.split("\n");
                for (var i = 0; i < lines.length; i++) {
                    if (lines[i].trim().indexOf("- #") === 0)
                        n++;
                }
                shim.fpHasFingers = hasDev && n > 0;
                shim.maybeArm();
            }
        }
        onExited: (code) => {
            if (code !== 0) {
                shim.fpHasFingers = false;
                shim.maybeArm();
            }
            listRetry.restart();
        }
    }

    // bounded re-probe: three tries, two and a half seconds apart, only while
    // the lock is actually held and we still have no fingers on record.
    Timer {
        id: listRetry
        interval: 2500
        property int tries: 0
        onTriggered: {
            if (shim.fpHasFingers || !shim.armWhenReady || tries >= 3)
                return;
            tries++;
            fpListProc.running = true;
        }
    }

    // ── PAM conversations ────────────────────────────────────────────────────
    // TWO independent conversations race for the unlock — pam_fprintd's man
    // page: serialised stacks ⇒ separate conversations for parallel methods.
    // configDirectory is the lock's own assets/pam/, so no root edit of
    // /etc/pam.d is needed — both services ship with the lock screen itself.
    PamContext {
        id: pamFp
        config: "ryoku-lock"
        configDirectory: Quickshell.shellDir + "/assets/pam"

        // Fingerprint conversation. timeout=-1 (in the service file) keeps
        // it scanning for the whole lock; only 3 misreads or an error end it.
        // A genuine misread re-arms after 1s; a scan that dies instantly is a
        // held claim (#243) and backs off instead of looping. It never prompts,
        // so it can never leave a pending response blocking the result.
        onCompleted: (result) => {
            if (shim.unlocked)
                return;
            if (result === PamResult.Success) {
                shim.unlocked = true;
                // This conversation never prompts, so a key can never reach
                // it: a success here is the sensor winning, whatever the
                // password field holds.
                shim.fingerprintUnlock = true;
                shim.fingerprintState = "success";
                pamPw.abort();
                shim.sddm.loginSucceeded();
                Quickshell.execDetached(["loginctl", "unlock-session"]);
            } else {
                shim.noteFpFailure();
            }
        }
        onError: (error) => {
            if (!shim.armWhenReady || shim.unlocked)
                return;
            shim.noteFpFailure();
        }
    }

    PamContext {
        id: pamPw
        property string pendingPassword: ""
        config: "ryoku-lock-pw"
        configDirectory: Quickshell.shellDir + "/assets/pam"

        // The password prompt is live from lock time: feed the stashed key
        // the moment PAM asks (usually PAM is already asking by the time
        // login() stashes it, so the response goes out on the same tick).
        onResponseRequiredChanged: {
            if (responseRequired && pendingPassword !== "") {
                shim.fpTyped = true;
                respond(pendingPassword);
                pendingPassword = "";
            }
        }

        onCompleted: (result) => {
            if (shim.unlocked)
                return;
            if (result === PamResult.Success) {
                shim.unlocked = true;
                shim.fingerprintUnlock = false;
                shim.fingerprintState = "success";
                pamFp.abort();
                shim.sddm.loginSucceeded();
                Quickshell.execDetached(["loginctl", "unlock-session"]);
            } else {
                // Wrong password: shake the field — the sensor conversation
                // is untouched and keeps scanning — then re-prompt right
                // away so the retry answers instantly.
                shim.fpTyped = false;
                shim.sddm.loginFailed();
                if (shim.armWhenReady)
                    shim.startPw();
            }
        }
        onError: (error) => {
            if (!shim.armWhenReady || shim.unlocked)
                return;
            if (shim.fpTyped || pamPw.pendingPassword !== "")
                shim.sddm.loginFailed();
            shim.fpTyped = false;
            shim.startPw();
        }
    }

    // A genuine misread (three touches) is re-armed after a short settle; a
    // scan that dies almost instantly is fprintd refusing the Claim, so it is
    // counted and, after three, the sensor is parked as "unavailable" with an
    // exponential re-probe. The password conversation is untouched throughout,
    // so unlocking is never blocked — only the misleading red loop stops.
    function noteFpFailure() {
        if (!shim.armWhenReady || shim.unlocked)
            return;
        var instant = shim.fpArmTs > 0 && (Date.now() - shim.fpArmTs) < shim.fpInstantFailMs;
        if (!instant) {
            shim.fpFastFails = 0;
            shim.fingerprintState = "fail";
            rearmTimer.restart();
            return;
        }
        shim.fpFastFails++;
        if (shim.fpFastFails < 3) {
            // A couple of instant refusals can be a device still settling after
            // resume; give it the normal 1s before concluding it is held.
            shim.fingerprintState = "fail";
            rearmTimer.restart();
            return;
        }
        shim.fpUnavailable = true;
        shim.fingerprintState = "unavailable";
        console.warn("[fp] sensor claim held; parking fingerprint, backing off", shim.fpBackoffMs, "ms");
        fpProbeTimer.restart();
    }

    // settle before rescanning so the old verifier releases the claim.
    Timer {
        id: rearmTimer
        interval: 1000
        onTriggered: {
            if (!shim.armWhenReady || pamFp.active)
                return;
            shim.startPw();
            if (shim.fingerprintReady)
                shim.armFingerprint();
        }
    }

    // While the sensor is parked, re-probe on a doubling backoff (5s → 10s →
    // 20s, capped at 60s) so a device that recovers on its own comes back
    // without a per-second D-Bus/journal storm.
    Timer {
        id: fpProbeTimer
        interval: shim.fpBackoffMs
        onTriggered: {
            if (!shim.armWhenReady || shim.unlocked || pamFp.active)
                return;
            shim.fpBackoffMs = Math.min(60000, shim.fpBackoffMs * 2);
            // One instant failure re-parks (threshold is 3); a real touch resets
            // the count in noteFpFailure, so a recovered device is not punished.
            shim.fpFastFails = 2;
            shim.fingerprintState = "idle";
            if (shim.fingerprintReady)
                shim.armFingerprint();
        }
    }

    // ── fingerprint control functions ───────────────────────────────────────

    // Clear any orphaned verifier first (armPrepProc), then arm on its exit.
    function armFingerprint() {
        if (!shim.fingerprintReady || pamFp.active || shim.armPending)
            return;
        shim.armPending = true;
        armPrepProc.running = true;
    }

    function armFingerprintNow() {
        if (!shim.fingerprintReady || pamFp.active)
            return;
        pamFp.user = Quickshell.env("USER") || "traveler";
        shim.fingerprintUnlock = false;
        shim.fingerprintState = "scanning";
        shim.fpArmTs = Date.now();
        // start() returns false when the config dir/file or user cannot be
        // resolved; surface that instead of failing silently.
        var started = pamFp.start();
        if (!started)
            console.warn("[fp] pamFp.start() failed: config=", pamFp.config,
                "dir=", pamFp.configDirectory, "user=", pamFp.user);
        shim.startPw();
    }

    // resetAuth: aborts any active PAM conversation and resets state.
    // Called when the lock surface is unlocked.
    function resetAuth() {
        shim.unlocked = false;
        shim.fingerprintUnlock = false;
        shim.fpTyped = false;
        shim.fpFastFails = 0;
        shim.fpUnavailable = false;
        shim.fpBackoffMs = 5000;
        shim.fpArmTs = 0;
        fpProbeTimer.stop();
        rearmTimer.stop();
        pamFp.abort();
        pamPw.abort();
        if (shim.fingerprintState !== "idle")
            shim.fingerprintState = "idle";
    }

    function maybeArm() {
        if (!shim.armWhenReady)
            return;
        shim.startPw();
        if (shim.fingerprintReady && !pamFp.active)
            shim.armFingerprint();
    }

    // The password conversation lives from lock time until unlock,
    // independent of the sensor, so a typed key is answered instantly.
    // Idempotent: safe to call from every probe and every re-arm.
    function startPw() {
        if (!shim.armWhenReady || shim.unlocked || pamPw.active)
            return;
        pamPw.user = Quickshell.env("USER") || "traveler";
        pamPw.pendingPassword = "";
        var started = pamPw.start();
        if (!started)
            console.warn("[fp] pamPw.start() failed: config=", pamPw.config,
                "dir=", pamPw.configDirectory, "user=", pamPw.user);
    }

    // ── initialization ──────────────────────────────────────────────────────
    onThemePathChanged: loadConfig(themePath)
    Component.onCompleted: {
        sessionEnumerator.running = true;
        hostnameProc.running = true;
        fpToggleProc.running = true;
        fpListProc.running = true;
    }
}
