import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// sign-in and security as a Changes store: the keyring, fingerprint and security
// key backends are the truth; every write re-reads them, so a row shows what applied
QtObject {
    id: store

    readonly property string user: Quickshell.env("USER") || ""

    // keyring (`ryoku keyring status --json`)
    property bool kloaded: false
    property string kmode: ""
    property bool kdaemon: false
    property string kdefFormat: ""
    // automatic sign-in leaves no login password to open the keyring with
    property bool kautologin: false
    property string kreadError: ""
    // "" or the mode that waits for the keyring's password
    property string convertFor: ""

    // fingerprint (fprintd as this user, the lock's preference file, the host seam)
    property bool floaded: false
    property bool fdaemon: false
    property bool fready: false
    property string fdeviceName: ""
    property var fingers: []
    property var fnames: ({})
    property string freadError: ""
    property bool fpEnabled: true
    property bool fpamLoaded: false
    property bool fpamSupported: false
    property string fpamReason: ""
    property string fpamReadError: ""
    property var fpam: ({ sudo: false, sddm: false, "polkit-1": false })

    // security key (`ryoku security-key status --json`)
    property bool skLoaded: false
    property var sk: ({})
    property string skReadError: ""

    // key -> value being written, shown until the backend answers
    property var pendingValues: ({})
    // group -> true while a write of that group runs: keyring, fingerprint, key
    property var busy: ({})
    // group -> what its running write changes (a store key, or keyring.convert, fingerprint.delete, key.remove)
    property var busyWhat: ({})
    // group -> the running command, and the groups whose wait was stopped
    property var live: ({})
    property var stopped: ({})
    // reads in flight, so a second read waits for the first
    property var reading: ({})

    readonly property var fpTargets: ({ "fingerprint.sudo": "sudo", "fingerprint.sddm": "sddm", "fingerprint.polkit": "polkit-1" })
    readonly property var skTargets: ({ "key.sudo": "sudo", "key.polkit": "polkit", "key.login": "login", "key.lock": "lock" })
    readonly property var skFlags: ({ "key.touch": "touch-required", "key.pin": "pin-verification", "key.uv": "user-verification" })

    function groupOf(key) {
        return key.split(".")[0];
    }

    function applied(key) {
        if (key === "keyring.mode")
            return store.kloaded ? store.kmode : undefined;
        if (key === "keyring.blocked")
            return store.kloaded ? store.kdefFormat === "encrypted" : undefined;
        if (key === "fingerprint.lock")
            return store.fpEnabled;
        if (key === "fingerprint.pam")
            return store.fpamLoaded ? store.fpamSupported : undefined;
        if (store.fpTargets[key] !== undefined)
            return store.fpamLoaded && store.fpamSupported ? store.fpam[store.fpTargets[key]] === true : undefined;
        if (!store.skLoaded)
            return undefined;
        var s = store.sk;
        switch (key) {
        case "key.supported": return s.supported === true;
        case "key.enrolled": return s.supported === true && s.enrolled === true;
        case "key.lockReady": return s.supported === true && s.enrolled === true && s.lockSupported === true;
        case "key.sudo": return s.sudo === true;
        case "key.polkit": return s.polkit === true;
        case "key.login": return s.login === true;
        case "key.lock": return s.lock === true;
        case "key.mode": return s.authMode === "mfa" ? "mfa" : "either";
        case "key.touch": return s.touchRequired !== false;
        case "key.pin": return s.pinVerification === true;
        case "key.uv": return s.userVerification === true;
        }
        return undefined;
    }

    function read(key) {
        if (store.pendingValues[key] !== undefined)
            return store.pendingValues[key];
        return store.applied(key);
    }

    function setBusy(group, on, what) {
        var b = Object.assign({}, store.busy), w = Object.assign({}, store.busyWhat);
        if (on) {
            b[group] = true;
            w[group] = what || group;
            store.halted(group);
        } else {
            delete b[group];
            delete w[group];
        }
        store.busyWhat = w;
        store.busy = b;
        Changes.touch();
    }

    // stop waiting for a write (a password prompt nobody answers): its command ends
    function cancel(group) {
        if (!store.busy[group])
            return;
        var s = Object.assign({}, store.stopped);
        s[group] = true;
        store.stopped = s;
        var p = store.live[group];
        if (p)
            p.running = false;
    }

    // whether the group's last write was stopped; asking clears it
    function halted(group) {
        if (!store.stopped[group])
            return false;
        var s = Object.assign({}, store.stopped);
        delete s[group];
        store.stopped = s;
        return true;
    }

    function track(group, p) {
        var l = Object.assign({}, store.live);
        if (p)
            l[group] = p;
        else
            delete l[group];
        store.live = l;
    }

    function stoppedText() {
        return I18n.tr("Stopped before it finished. Retry when you are ready.");
    }

    function setPending(key, v) {
        var p = Object.assign({}, store.pendingValues);
        if (v === undefined)
            delete p[key];
        else
            p[key] = v;
        store.pendingValues = p;
        Changes.touch();
    }

    // a tool's own name or an "error:" tag at a line start is not news to the reader
    function plain(text) {
        return String(text || "").trim().replace(/^(?:ryoku[\w-]*|fprintd[\w-]*|error):\s*/gim, "");
    }

    // a privileged step that failed: say what happened and what to do next
    function failure(code, err, what) {
        var e = store.plain(err);
        // the frame's reason is a full sentence of its own
        if (code === -1)
            return I18n.tr("%1 did not change. %2").arg(what).arg(e);
        if (code === 126 || code === 127 || /dismissed|not authorized|authentication failed/i.test(e))
            return I18n.tr("%1 did not change because the admin prompt was cancelled. Retry to ask for your password again.").arg(what);
        if (e === "")
            return I18n.tr("%1 did not change. Retry, and if it keeps failing, run ryoku doctor.").arg(what);
        return I18n.tr("%1 did not change: %2. Retry once that is fixed.").arg(what).arg(e.replace(/[.\s]+$/, ""));
    }

    // run argv, then call back exactly once with (ok, code, out, err); code -1 when it
    // cannot start; input goes to stdin once; a group lets cancel(group) stop it
    function run(argv, input, callback, group) {
        var finish = function (ok, code, out, err) {
            if (group)
                store.track(group, null);
            callback(ok, code, out, err);
        };
        if (!input) {
            var c = Changes.run(argv, function (ok, out, err, code) {
                finish(ok, code, out, err);
            });
            if (group && store.busy[group])
                store.track(group, c);
            return;
        }
        var p = store.runner.createObject(store, { command: argv, input: input });
        p.done.connect(function (code, out, err) {
            finish(code === 0, code, out, err);
            p.destroy();
        });
        if (group)
            store.track(group, p);
        p.running = true;
    }

    // a read with a deadline; done(ok, out, why) where why is a full sentence when it could not run
    function readCommand(name, argv, done) {
        if (store.reading[name])
            return;
        var r = Object.assign({}, store.reading);
        r[name] = true;
        store.reading = r;
        Changes.run(argv, function (ok, out, err, code) {
            var left = Object.assign({}, store.reading);
            delete left[name];
            store.reading = left;
            done(ok, out, code === -1 ? String(err || "").trim() : "");
        }, { timeout: 15000 });
    }

    function write(key, v, done) {
        var group = store.groupOf(key);
        if (store.busy[group]) {
            done(false, I18n.tr("Another change here is still waiting. Try again when it finishes."));
            return;
        }
        if (key === "keyring.mode") {
            store.writeKeyring(String(v), done);
            return;
        }
        if (key === "fingerprint.lock") {
            store.writeFingerprintLock(v === true, done);
            return;
        }
        var argv = null, what = "";
        if (store.fpTargets[key] !== undefined) {
            argv = ["ryoku-host", "auth", "fingerprint", "set", store.fpTargets[key], v === true ? "on" : "off"];
            what = I18n.tr("Fingerprint use");
        } else if (store.skTargets[key] !== undefined) {
            argv = ["ryoku", "security-key", "set", store.skTargets[key], v === true ? "on" : "off"];
            what = I18n.tr("Security key use");
        } else if (key === "key.mode") {
            argv = ["ryoku", "security-key", "set", "mode", v === "mfa" ? "mfa" : "either"];
            what = I18n.tr("How a key signs you in");
        } else if (store.skFlags[key] !== undefined) {
            argv = ["ryoku", "security-key", "set", store.skFlags[key], v === true ? "on" : "off"];
            what = I18n.tr("The security key rule");
        }
        if (!argv) {
            done(true, "");
            return;
        }
        if (store.applied(key) === v) {
            done(true, "");
            return;
        }
        store.setPending(key, v);
        store.setBusy(group, true, key);
        store.run(argv, "", function (ok, code, out, err) {
            store.setPending(key, undefined);
            store.setBusy(group, false);
            done(ok, ok ? "" : store.halted(group) ? store.stoppedText() : store.failure(code, err || out, what));
            if (group === "fingerprint")
                store.readFingerprintPam();
            else
                store.readKey();
        }, group);
    }

    // keyring

    function writeKeyring(mode, done) {
        // never-ask on a keyring with a password cannot take effect until it is blank
        if (mode === "never-ask" && store.kdefFormat === "encrypted") {
            store.convertFor = "never-ask";
            Changes.touch();
            done(false, I18n.tr("Your keyring has a password, so Never ask cannot take effect yet. Type that password below to remove it, or start fresh."));
            return;
        }
        store.convertFor = "";
        if (mode === store.kmode) {
            Changes.touch();
            done(true, "");
            return;
        }
        store.setPending("keyring.mode", mode);
        store.setBusy("keyring", true, "keyring.mode");
        store.run(["ryoku", "keyring", "set", mode], "", function (ok, code, out, err) {
            store.setPending("keyring.mode", undefined);
            store.setBusy("keyring", false);
            done(ok, ok ? "" : store.halted("keyring") ? store.stoppedText() : store.failure(code, err, I18n.tr("The keyring")));
            store.readKeyring();
        }, "keyring");
    }

    // remove the keyring's password, or start fresh; callback(ok, message)
    function convertKeyring(password, callback) {
        store.finishKeyring(["ryoku", "keyring", "set", store.convertFor, "--convert", "--password-stdin"], password + "\n", callback);
    }

    function resetKeyring(callback) {
        store.finishKeyring(["ryoku", "keyring", "set", store.convertFor, "--reset"], "", callback);
    }

    function finishKeyring(argv, input, callback) {
        if (store.busy.keyring || store.convertFor === "")
            return;
        store.setBusy("keyring", true, "keyring.convert");
        store.run(argv, input, function (ok, code, out, err) {
            store.setBusy("keyring", false);
            if (ok) {
                store.convertFor = "";
                Changes.setError({ store: "signin", key: "keyring.mode" }, "", undefined);
            }
            var why = String(err || "").trim().replace(/[.\s]+$/, "");
            var msg = ok ? ""
                : store.halted("keyring") ? store.stoppedText()
                : input.length > 0 && code !== 126 && code !== 127 && code !== -1
                ? I18n.tr("The keyring did not take that password (%1). Check it and try again.").arg(why || ("exit " + code))
                : store.failure(code, err, I18n.tr("The keyring"));
            callback(ok, msg);
            store.readKeyring();
        }, "keyring");
    }

    function readKeyring() {
        store.readCommand("keyring", ["ryoku", "keyring", "status", "--json"], function (ok, out, why) {
            try {
                if (!ok)
                    throw why;
                var o = JSON.parse(out);
                store.kmode = o.mode || "";
                store.kdaemon = o.daemon_alive === true;
                store.kautologin = o.autologin === true;
                var def = (o.keyrings || []).find(k => k.role === "default");
                store.kdefFormat = def ? def.format : "";
                store.kreadError = "";
                // never-ask on paper, a password on disk: open the fix without waiting for a click
                if (store.kmode === "never-ask" && store.kdefFormat === "encrypted" && !store.busy.keyring)
                    store.convertFor = "never-ask";
                else if (store.kdefFormat !== "encrypted")
                    store.convertFor = "";
                store.kloaded = true;
            } catch (e) {
                store.kreadError = why !== "" ? I18n.tr("Could not read the keyring's state. %1").arg(why)
                    : I18n.tr("Could not read the keyring's state. Retry, and if it keeps failing, run ryoku doctor.");
            }
            Changes.touch();
        });
    }

    // fingerprint

    function writeFingerprintLock(on, done) {
        store.setBusy("fingerprint", true, "fingerprint.lock");
        store.run(["sh", "-c", "mkdir -p \"$HOME/.config/qylock\" && printf '%s\\n' \"$1\" > \"$HOME/.config/qylock/fingerprint\"", "sh", on ? "on" : "off"], "",
                  function (ok, code, out, err) {
            store.setBusy("fingerprint", false);
            if (ok)
                store.fpEnabled = on;
            Changes.touch();
            done(ok, ok ? "" : store.halted("fingerprint") ? store.stoppedText()
                : I18n.tr("Could not save the lock screen's fingerprint setting: %1. Retry.").arg(err.trim() || ("exit " + code)));
        }, "fingerprint");
    }

    function readFingerprints() {
        store.freadError = "";
        fpTimeout.restart();
        if (!store.flistProc.running)
            store.flistProc.running = true;
        store.run(["sh", "-c", "cat \"$HOME/.config/qylock/fingerprint\" 2>/dev/null || echo on"], "", function (ok, code, out) {
            var v = out.trim().toLowerCase();
            store.fpEnabled = !(v === "off" || v === "0" || v === "false");
            Changes.touch();
        });
        store.run(["sh", "-c", "cat \"$HOME/.config/qylock/fingerprints.json\" 2>/dev/null || echo '{}'"], "", function (ok, code, out) {
            try {
                store.fnames = JSON.parse(out || "{}") || {};
            } catch (e) {
                store.fnames = {};
            }
        });
        store.readFingerprintPam();
    }

    function readFingerprintPam() {
        store.readCommand("fpam", ["ryoku-host", "auth", "fingerprint", "status"], function (ok, out, why) {
            try {
                if (!ok)
                    throw why;
                var o = JSON.parse(out || "{}");
                var t = o.targets || {};
                store.fpamSupported = o.supported === true;
                store.fpamReason = String(o.reason || "");
                store.fpamReadError = "";
                store.fpam = { sudo: t.sudo === true, sddm: t.sddm === true, "polkit-1": t["polkit-1"] === true };
            } catch (e) {
                store.fpamSupported = false;
                store.fpamReason = "";
                store.fpamReadError = why !== "" ? I18n.tr("Could not read where fingerprints are used. %1").arg(why)
                    : I18n.tr("Could not read where fingerprints are used. Retry, and if it keeps failing, run ryoku doctor.");
            }
            store.fpamLoaded = true;
            Changes.touch();
        });
    }

    function parseFingers(text) {
        fpTimeout.stop();
        var t = text || "";
        var dn = t.match(/on (.+?) \((?:press|swipe)\):/i);
        store.fdeviceName = dn ? dn[1] : "";
        var fingers = [];
        var lines = t.split("\n");
        for (var i = 0; i < lines.length; i++) {
            var s = lines[i].trim();
            if (s.indexOf("- #") === 0) {
                var at = s.indexOf(":", 3);
                if (at > -1)
                    fingers.push(s.slice(at + 1).trim());
            }
        }
        // the header counts devices even when there are none
        var fm = t.match(/found\s+(\d+)\s+devices?/i);
        var count = fm ? parseInt(fm[1], 10) : 0;
        store.fready = t.indexOf("Using device") !== -1 || t.indexOf("Device at") !== -1 || count > 0;
        store.fdaemon = store.fready && t.trim() !== "" && t.indexOf("no devices") === -1;
        store.fingers = fingers;
        store.floaded = true;
        // names of prints that are gone go too
        var names = Object.assign({}, store.fnames), stale = false;
        for (var k in names)
            if (fingers.indexOf(k) === -1) {
                delete names[k];
                stale = true;
            }
        if (stale && store.fdaemon)
            store.writeNames(names, null);
        Changes.touch();
    }

    function writeNames(names, callback) {
        store.fnames = names;
        store.run(["sh", "-c", "mkdir -p \"$HOME/.config/qylock\" && cat > \"$HOME/.config/qylock/fingerprints.json\""], JSON.stringify(names),
                  function (ok, code, out, err) {
            if (callback)
                callback(ok, ok ? "" : I18n.tr("Could not save the name: %1. Retry.").arg(err.trim() || ("exit " + code)));
        });
    }

    function nameFinger(finger, name, callback) {
        var names = Object.assign({}, store.fnames);
        var n = String(name || "").trim();
        if (n === "")
            delete names[finger];
        else
            names[finger] = n;
        store.writeNames(names, callback);
    }

    // finger "" removes every print; callback(ok, message)
    function deleteFinger(finger, callback) {
        if (store.busy.fingerprint)
            return;
        store.setBusy("fingerprint", true, "fingerprint.delete");
        store.run(["fprintd-delete", store.user].concat(finger ? ["-f", finger] : []), "", function (ok, code, out, err) {
            store.setBusy("fingerprint", false);
            if (ok) {
                var names = finger ? Object.assign({}, store.fnames) : {};
                if (finger)
                    delete names[finger];
                store.writeNames(names, null);
            }
            callback(ok, ok ? "" : store.halted("fingerprint") ? store.stoppedText()
                : I18n.tr("The fingerprint was not removed: %1. Retry.").arg(store.plain(err || out).replace(/[.\s]+$/, "") || ("exit " + code)));
            store.readFingerprints();
        }, "fingerprint");
    }

    // security key

    function readKey() {
        store.readCommand("key", ["ryoku", "security-key", "status", "--json"], function (ok, out, why) {
            try {
                if (!ok)
                    throw why;
                store.sk = JSON.parse(out);
                store.skReadError = "";
            } catch (e) {
                store.sk = {};
                store.skReadError = why !== "" ? I18n.tr("Could not read the security key's state. %1").arg(why)
                    : I18n.tr("Could not read the security key's state. Retry, and if it keeps failing, run ryoku doctor.");
            }
            store.skLoaded = true;
            Changes.touch();
        });
    }

    function removeKey(id, callback) {
        if (store.busy.key)
            return;
        store.setBusy("key", true, "key.remove");
        store.run(["ryoku", "security-key", "remove", String(id)], "", function (ok, code, out, err) {
            store.setBusy("key", false);
            callback(ok, ok ? "" : store.halted("key") ? store.stoppedText() : store.failure(code, err || out, I18n.tr("The security key list")));
            store.readKey();
        }, "key");
    }

    function reload() {
        store.readKeyring();
        store.readFingerprints();
        store.readKey();
    }

    property Component runner: Component {
        Process {
            id: proc
            property string input: ""
            property bool launched: false
            signal done(int code, string out, string err)
            stdinEnabled: true
            stdout: StdioCollector { id: rout }
            stderr: StdioCollector { id: rerr }
            onStarted: {
                proc.launched = true;
                proc.write(proc.input);
                proc.input = "";
            }
            onExited: code => proc.done(code, rout.text, rerr.text)
            // a program that cannot start reports no exit, only that it stopped running
            onRunningChanged: {
                if (!proc.running && !proc.launched)
                    proc.done(-1, "", Changes.missingText(String(proc.command[0])));
            }
        }
    }

    property Process flistProc: Process {
        command: ["fprintd-list", store.user]
        stdout: StdioCollector { id: flistOut }
        onExited: code => {
            if (code !== 0 && flistOut.text.trim() === "") {
                fpTimeout.stop();
                store.fdaemon = false;
                store.fready = false;
                store.fingers = [];
                store.floaded = true;
                store.freadError = I18n.tr("The fingerprint service is not running.");
                Changes.touch();
                return;
            }
            store.parseFingers(flistOut.text);
        }
    }

    // fprintd starts on demand and can hang; fail to a clear state
    property Timer fpTimer: Timer {
        id: fpTimeout
        interval: 6000
        onTriggered: {
            if (store.floaded && !store.flistProc.running)
                return;
            store.floaded = true;
            store.fdaemon = false;
            store.fready = false;
            store.fingers = [];
            store.freadError = I18n.tr("The fingerprint service is not answering.");
            Changes.touch();
        }
    }

    Component.onCompleted: {
        Changes.registerStore("signin", store);
        store.reload();
    }
}
