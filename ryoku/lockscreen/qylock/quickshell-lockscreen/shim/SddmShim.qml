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
//   1. lock_shell.qml publishes proof after WlSessionLock becomes secure
//   2. Both PAM conversations are armed only while this login1 session is active
//   3. The shim probes fprintd-list + ~/.config/qylock/fingerprint
//   4. Stale fprintd-verify processes are cleared, then pamFp (fingerprint)
//      runs beside pamPw (typed password)
//   5. First conversation to succeed asks the root lock state machine to unlock
//   6. Leaving the foreground aborts both conversations and cancels any pending
//      authenticated reveal
//   7. A failed scan re-arms after 1s; a wrong password re-prompts immediately
//      without starving the other method; a password stack that fails before
//      it ever prompts re-arms on a doubling delay instead of looping
//   8. With a security key enabled (`ryoku security-key set lock on`), a third
//      conversation (pamKey) runs from the user's own PAM file. It arms when
//      the lock secures and when a key is plugged in. A touch-only key unlocks
//      beside the password; a key that asks for a PIN takes the field until
//      the user picks the password (sddm.usePassword), a PIN misses, or the
//      touch times out. Only a skin that draws sddm.keyState (sddm.keyPrompts)
//      gets the PIN; under any other skin the request is dropped unseen

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
    property bool loginSessionActive: false  // only the foreground VT may authenticate
    readonly property bool fingerprintReady: fpEnabled && fpHasFingers && loginSessionActive
    property string fingerprintState: "idle" // idle | scanning | success | fail | unavailable
    property bool fingerprintUnlock: false   // true if sensor won (not typed)
    property bool armWhenReady: false        // secure proof is published; active session may arm
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

    // Password re-arm gap after a conversation that ended before any prompt:
    // the stack itself cannot run, so nothing typed can fix it. Doubles to a
    // 30s cap and drops back once the stack reaches a prompt again.
    property int pwBackoffMs: 1000

    // ── security key state ──────────────────────────────────────────────────
    readonly property string keyConfigDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/qylock/pam"
    property bool keyEnabled: false          // the user's ryoku-lock-key file exists
    property bool keyTyped: false            // a PIN was fed to the key conversation
    property bool keyPasswordMode: false     // the user, a miss, or a timeout chose the password
    property string keyState: "off"          // off | pin | verifying | touch
    property real keyArmTs: 0                // ms epoch the key conversation started

    // arm the moment the lock secures; probes alone race and lose it.
    onArmWhenReadyChanged: {
        if (shim.armWhenReady)
            shim.maybeArm();
    }

    function applySessionActivity(value) {
        var active = String(value).trim() === "yes";
        if (shim.loginSessionActive === active)
            return;
        shim.loginSessionActive = active;
        if (active) {
            shim.maybeArm();
            return;
        }
        shim.armPending = false;
        armPrepProc.running = false;
        rearmTimer.stop();
        fpProbeTimer.stop();
        pwRetryTimer.stop();
        pamFp.abort();
        pamPw.pendingPassword = "";
        pamPw.abort();
        keyHotplugDelay.stop();
        keyRearmTimer.stop();
        shim.keyTyped = false;
        pamKey.abort();
        shim.keyState = "off";
        shim.unlocked = false;
        shim.fingerprintState = "idle";
    }

    Process {
        id: sessionActivityProc
        running: true
        command: [
            "bash", "-c",
            "emit_active() { " +
            "v=$(loginctl show-session \"$XDG_SESSION_ID\" -p Active --value 2>/dev/null) || return 1; " +
            "case \"$v\" in yes|no) printf '%s\\n' \"$v\";; *) return 1;; esac; }; " +
            "probe() { emit_active || printf 'unknown\\n'; }; " +
            "(dbus-monitor --system \"type='signal',sender='org.freedesktop.login1'," +
            "path_namespace='/org/freedesktop/login1/session'," +
            "interface='org.freedesktop.DBus.Properties',member='PropertiesChanged'," +
            "arg0='org.freedesktop.login1.Session'\" 2>/dev/null | " +
            "while IFS= read -r line; do case \"$line\" in *PropertiesChanged*) probe;; esac; done) & " +
            "monitor=$!; trap 'kill \"$monitor\" 2>/dev/null || true' EXIT; " +
            "while kill -0 \"$monitor\" 2>/dev/null; do probe; sleep 0.5; done; wait \"$monitor\"; exit 1"
        ]
        stdout: SplitParser {
            onRead: (line) => shim.applySessionActivity(line)
        }
        onExited: () => {
            shim.applySessionActivity("no");
            sessionActivityRestart.restart();
        }
    }

    Timer {
        id: sessionActivityRestart
        interval: 1000
        onTriggered: sessionActivityProc.running = true
    }

    // orphaned verifiers hold the sensor claim; clear them before arming.
    Process {
        id: armPrepProc
        command: ["bash", "-c", "pkill -u \"$USER\" -x fprintd-verify 2>/dev/null; sleep 0.2"]
        onExited: () => {
            if (!shim.armPending)
                return;
            shim.armPending = false;
            if (shim.loginSessionActive)
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
        signal keyTimedOut()
        // A skin that draws keyState sets this; until one does, the key may
        // unlock by touch but never takes typed input.
        property bool keyPrompts: false
        readonly property bool keyReady: shim.keyEnabled
        readonly property string keyState: shim.keyState
        readonly property bool keyPasswordMode: shim.keyPasswordMode

        function usePassword() {
            shim.keyPasswordMode = true;
            shim.keyTyped = false;
            keyRearmTimer.stop();
            pamKey.abort();
            shim.keyState = "off";
        }
        function useKey() {
            shim.keyPasswordMode = false;
            shim.armKey();
        }
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
            if (!shim.armWhenReady || !shim.loginSessionActive || shim.unlocked) {
                pamPw.pendingPassword = "";
                shim.sddm.loginFailed();
                return;
            }
            // While the key asks for its PIN, the field answers the key; one
            // wrong PIN drops to the password.
            if (shim.keyState === "pin" && password !== "") {
                shim.keyTyped = true;
                shim.keyState = "verifying";
                pamKey.respond(password);
                return;
            }
            if (password === "" && shim.keyEnabled)
                return;
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
            // a typed key gets its attempt now, not after the backoff
            pwRetryTimer.stop();
            shim.startPw();
        }

        // ryoku-host picks systemctl or loginctl for the running init, like
        // the shell's power menus; systemd's loginctl has no power verbs.
        function reboot() { Quickshell.execDetached(["ryoku-host", "power", "reboot"]); }
        function powerOff() { Quickshell.execDetached(["ryoku-host", "power", "poweroff"]); }
        function suspend() { Quickshell.execDetached(["ryoku-shell", "suspend"]); }
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
            if (result === PamResult.Success && shim.armWhenReady && shim.loginSessionActive) {
                shim.unlocked = true;
                // This conversation never prompts, so a key can never reach
                // it: a success here is the sensor winning, whatever the
                // password field holds.
                shim.fingerprintUnlock = true;
                shim.fingerprintState = "success";
                pamPw.abort();
                pamKey.abort();
                shim.sddm.loginSucceeded();
            } else if (result === PamResult.Success) {
                shim.unlocked = false;
                shim.fingerprintState = "idle";
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

        // Once the secure proof starts this conversation, feed a stashed key
        // the moment PAM asks. Usually PAM is already asking by the time
        // login() stashes it, so the response goes out on the same tick.
        onResponseRequiredChanged: {
            if (responseRequired)
                shim.pwBackoffMs = 1000;
            if (responseRequired && pendingPassword !== "") {
                shim.fpTyped = true;
                respond(pendingPassword);
                pendingPassword = "";
            }
        }

        onCompleted: (result) => {
            // Quickshell follows error() with completed(Error) for the same
            // conversation; onError already handled it, and a second pass
            // would read the failure as untyped.
            if (shim.unlocked || result === PamResult.Error)
                return;
            if (result === PamResult.Success && shim.armWhenReady && shim.loginSessionActive) {
                shim.unlocked = true;
                shim.fingerprintUnlock = false;
                shim.fingerprintState = "success";
                pamFp.abort();
                pamKey.abort();
                shim.sddm.loginSucceeded();
            } else if (result === PamResult.Success) {
                shim.unlocked = false;
                shim.fingerprintState = "idle";
            } else {
                shim.notePwFailure();
            }
        }
        onError: (error) => {
            if (!shim.armWhenReady || shim.unlocked)
                return;
            shim.notePwFailure();
        }
    }

    // Security key conversation, from the file `ryoku security-key set lock`
    // writes: pam_u2f, then deny. It starts from armKey() only: the secure
    // lock, a plug-in, sddm.useKey(), or a touch-only wait re-arming itself.
    PamContext {
        id: pamKey
        config: "ryoku-lock-key"
        configDirectory: shim.keyConfigDir

        onPamMessage: {
            if (responseRequired) {
                if (!shim.sddm.keyPrompts) {
                    console.warn("[key] the lock skin does not draw key prompts; dropping the PIN request");
                    abort();
                    shim.keyState = "off";
                    return;
                }
                shim.keyState = "pin";
            } else if (!messageIsError && shim.keyState !== "verifying") {
                // pam_u2f's touch cue with no PIN asked first: a touch-only
                // key, which unlocks beside the password.
                shim.keyState = "touch";
            }
        }

        onCompleted: (result) => {
            if (shim.unlocked || result === PamResult.Error)
                return;
            if (result === PamResult.Success && shim.armWhenReady && shim.loginSessionActive) {
                shim.unlocked = true;
                // Skins that take the PIN hand it over without the wind-up,
                // so a key win always plays the sensor reveal.
                shim.fingerprintUnlock = true;
                shim.fingerprintState = "success";
                shim.keyTyped = false;
                shim.keyState = "off";
                pamFp.abort();
                pamPw.abort();
                shim.sddm.loginSucceeded();
                return;
            }
            shim.parkKey();
        }
        onError: (error) => {
            if (!shim.armWhenReady || shim.unlocked)
                return;
            shim.parkKey();
        }
    }

    // pam_u2f's own wait can outlast the user; the lock gives up first and
    // hands the field back to the password.
    Timer {
        id: keyTouchTimer
        interval: 10000
        running: shim.keyState === "verifying"
        onTriggered: {
            console.warn("[key] no touch within", interval, "ms; back to the password");
            shim.keyTyped = false;
            shim.keyPasswordMode = true;
            pamKey.abort();
            shim.keyState = "off";
            shim.sddm.keyTimedOut();
        }
    }

    // A wrong PIN shakes the field like a wrong password and hands it to the
    // password. A touch-only wait that ran its course re-arms; one that died at
    // once had no key to talk to, so it waits for the next plug-in instead.
    function parkKey() {
        var typed = shim.keyTyped;
        var waited = shim.keyState === "touch" && Date.now() - shim.keyArmTs > 1500;
        shim.keyTyped = false;
        shim.keyState = "off";
        if (typed) {
            shim.keyPasswordMode = true;
            shim.sddm.loginFailed();
        } else if (waited) {
            keyRearmTimer.restart();
        }
    }

    Timer {
        id: keyRearmTimer
        interval: 1000
        onTriggered: shim.armKey()
    }

    // The file is probed at every arm, so turning the key on or off applies to
    // the next lock without restarting it.
    Process {
        id: keyProbeProc
        command: ["test", "-r", shim.keyConfigDir + "/ryoku-lock-key"]
        onExited: (code) => {
            shim.keyEnabled = code === 0;
            if (!shim.keyEnabled || !shim.armWhenReady || !shim.loginSessionActive || shim.unlocked || pamKey.active)
                return;
            pamKey.user = Quickshell.env("USER") || "traveler";
            shim.keyArmTs = Date.now();
            if (!pamKey.start())
                console.warn("[key] pamKey.start() failed: config=", pamKey.config,
                    "dir=", pamKey.configDirectory, "user=", pamKey.user);
        }
    }

    // A key plugged in while locked arms the conversation, so its PIN prompt
    // appears without a keypress. udev reports the node after its ACL is set.
    Process {
        id: keyHotplugProc
        running: shim.keyEnabled && shim.armWhenReady && shim.loginSessionActive
        command: ["udevadm", "monitor", "--udev", "--subsystem-match=hidraw"]
        stdout: SplitParser {
            onRead: (line) => {
                if (line.indexOf(" add ") !== -1)
                    keyHotplugDelay.restart();
            }
        }
    }

    Timer {
        id: keyHotplugDelay
        interval: 500
        onTriggered: shim.armKey()
    }

    function armKey() {
        if (!shim.armWhenReady || !shim.loginSessionActive || shim.unlocked || shim.keyPasswordMode || pamKey.active || keyProbeProc.running)
            return;
        keyProbeProc.running = true;
    }

    // A key that reached PAM and failed is a wrong password: shake the field
    // (the sensor conversation is untouched and keeps scanning), then
    // re-prompt right away so the retry answers instantly. A conversation that
    // ended before any key reached it is the stack failing on its own, such as
    // a module that will not load or a service that denies outright. Starting
    // it again at once forks a PAM child per event-loop turn and shakes the
    // field with nothing typed, so it re-arms on a doubling delay instead.
    function notePwFailure() {
        var typed = shim.fpTyped;
        shim.fpTyped = false;
        if (typed || pamPw.pendingPassword !== "")
            shim.sddm.loginFailed();
        if (typed) {
            if (shim.armWhenReady)
                shim.startPw();
            return;
        }
        pamPw.pendingPassword = "";
        if (!shim.armWhenReady || !shim.loginSessionActive || shim.unlocked)
            return;
        console.warn("[pw] password conversation ended before any prompt; retrying in", shim.pwBackoffMs, "ms");
        pwRetryTimer.restart();
    }

    Timer {
        id: pwRetryTimer
        interval: shim.pwBackoffMs
        onTriggered: {
            shim.pwBackoffMs = Math.min(30000, shim.pwBackoffMs * 2);
            shim.startPw();
        }
    }

    // A genuine misread (three touches) is re-armed after a short settle; a
    // scan that dies almost instantly is fprintd refusing the Claim, so it is
    // counted and, after three, the sensor is parked as "unavailable" with an
    // exponential re-probe. While this session owns the foreground, the
    // independent password conversation remains available.
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
            if (!shim.armWhenReady || !shim.loginSessionActive || pamFp.active)
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
            if (!shim.armWhenReady || !shim.loginSessionActive || shim.unlocked || pamFp.active)
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
        if (!shim.fingerprintReady || !shim.loginSessionActive || pamFp.active || shim.armPending)
            return;
        shim.armPending = true;
        armPrepProc.running = true;
    }

    function armFingerprintNow() {
        if (!shim.fingerprintReady || !shim.loginSessionActive || pamFp.active)
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
        shim.pwBackoffMs = 1000;
        fpProbeTimer.stop();
        rearmTimer.stop();
        pwRetryTimer.stop();
        keyHotplugDelay.stop();
        keyRearmTimer.stop();
        shim.keyTyped = false;
        shim.keyPasswordMode = false;
        pamFp.abort();
        pamPw.abort();
        pamKey.abort();
        shim.keyState = "off";
        if (shim.fingerprintState !== "idle")
            shim.fingerprintState = "idle";
    }

    function maybeArm() {
        if (!shim.armWhenReady || !shim.loginSessionActive)
            return;
        shim.startPw();
        if (shim.fingerprintReady && !pamFp.active)
            shim.armFingerprint();
        shim.armKey();
    }

    // Once the secure proof is published and this login1 session owns the
    // foreground, keep the password conversation alive so a typed key is
    // answered instantly. Idempotent: safe from every probe and re-arm, and
    // a pending backoff holds them off until it fires.
    function startPw() {
        if (!shim.armWhenReady || !shim.loginSessionActive || shim.unlocked || pamPw.active || pwRetryTimer.running)
            return;
        pamPw.user = Quickshell.env("USER") || "traveler";
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
