import QtQuick
import Quickshell.Io
import Quickshell.Networking
import Ryoku.Ui.Singletons
import "../../Singletons"

// the Wi-Fi radio, the Wi-Fi backend, saved networks and the hotspot as a Changes
// store: NetworkManager is the truth, every write is a command, so undo works from any page
QtObject {
    id: store

    // the hotspot profile the old Settings created; kept so existing hotspots carry over
    readonly property string hotspotId: "RyokuHotspot"

    // saved Wi-Fi profiles, hotspot excluded: { uuid, name, ssid, autoconnect, timestamp, active, band }
    property var profiles: []
    property bool profilesRead: false
    // why the saved profiles could not be read, so the page can say so and offer Retry
    property string profilesError: ""
    property var hotspot: ({ uuid: "", exists: false, active: false })
    property string hotspotName: ""
    property string hotspotPassword: ""
    property string backend: ""
    // store keys with a write in flight, so rows can show they are working
    property var busy: ({})
    // the commands running for each store key, and the keys whose wait was cancelled
    property var live: ({})
    property var stopped: ({})

    readonly property var wifiDevice: {
        var list = Networking.devices ? Networking.devices.values : [];
        return list.find(d => d && d.type === DeviceType.Wifi) || null;
    }
    readonly property string iface: store.wifiDevice && store.wifiDevice.name ? store.wifiDevice.name : "wlan0"

    function read(key) {
        if (key === "enabled")
            return Networking.wifiEnabled === true;
        if (key === "backend")
            return store.backend === "" ? undefined : store.backend;
        if (key === "hotspot.on")
            return store.hotspot.active === true;
        if (key === "hotspot.name")
            return store.hotspotName;
        if (key === "hotspot.password")
            return store.hotspotPassword;
        if (key.indexOf("autoconnect.") === 0) {
            var p = store.profile(key.slice(12));
            return p ? p.autoconnect : undefined;
        }
        return undefined;
    }

    function mark(key, on) {
        var b = Object.assign({}, store.busy);
        if (on)
            b[key] = true;
        else
            delete b[key];
        store.busy = b;
    }

    function write(key, v, done) {
        store.mark(key, true);
        store.perform(key, v, function (ok, err) {
            store.mark(key, false);
            var halted = !!store.stopped[key];
            var s = Object.assign({}, store.stopped);
            delete s[key];
            store.stopped = s;
            done(ok, !ok && halted ? I18n.tr("Stopped before it finished. Retry when you are ready.") : err);
        });
    }

    // stop waiting for a write: its command ends and the row shows Retry
    function cancel(key) {
        if (!store.busy[key])
            return;
        var s = Object.assign({}, store.stopped);
        s[key] = true;
        store.stopped = s;
        (store.live[key] || []).forEach(function (p) {
            p.running = false;
        });
    }

    function track(key, p, on) {
        var l = Object.assign({}, store.live);
        var list = (l[key] || []).filter(x => x !== p);
        if (on)
            list.push(p);
        if (list.length > 0)
            l[key] = list;
        else
            delete l[key];
        store.live = l;
    }

    function perform(key, v, done) {
        if (key === "enabled") {
            store.run(["nmcli", "radio", "wifi", v ? "on" : "off"], done, "", key);
            return;
        }
        if (key === "backend") {
            store.run(["ryoku-host", "network", "wifi-backend", "set", String(v)], function (ok, err) {
                // networking restarts, so the answer is read back once it settles
                settle.restart();
                done(ok, err);
            }, "", key);
            return;
        }
        if (key.indexOf("autoconnect.") === 0) {
            var uuid = key.slice(12);
            store.patchProfile(uuid, { autoconnect: v === true });
            store.run(["nmcli", "connection", "modify", "uuid", uuid, "connection.autoconnect", v ? "yes" : "no"], function (ok, err) {
                store.refresh();
                done(ok, err);
            }, "", key);
            return;
        }
        if (key === "hotspot.on") {
            store.setHotspot(v === true, done, key);
            return;
        }
        if (key === "hotspot.name" || key === "hotspot.password") {
            if (key === "hotspot.name")
                store.hotspotName = String(v);
            else
                store.hotspotPassword = String(v);
            Changes.touch();
            store.storeHotspot(store.hotspot.active, done, key);
            return;
        }
        done(false, "");
    }

    function profile(uuid) {
        return store.profiles.find(p => p.uuid === uuid) || null;
    }

    function patchProfile(uuid, patch) {
        store.profiles = store.profiles.map(p => p.uuid === uuid ? Object.assign({}, p, patch) : p);
        Changes.touch();
    }

    // the saved profile for a network name, the most recently used first
    function profileFor(ssid) {
        var best = null;
        for (var i = 0; i < store.profiles.length; i++) {
            var p = store.profiles[i];
            if (p.ssid === ssid && (!best || p.timestamp > best.timestamp))
                best = p;
        }
        return best;
    }

    // 8 characters from an alphabet without look-alikes
    function generatePassword() {
        var cs = "abcdefghijkmnpqrstuvwxyz23456789", s = "";
        for (var i = 0; i < 8; i++)
            s += cs.charAt(Math.floor(Math.random() * cs.length));
        return s;
    }

    // the profile's argv: an edit when it exists, else a new access point that never joins on its own
    function hotspotArgv() {
        var name = store.hotspotName.length > 0 ? store.hotspotName : "Ryoku";
        if (store.hotspot.exists)
            return ["nmcli", "connection", "modify", "uuid", store.hotspot.uuid,
                    "802-11-wireless.ssid", name, "802-11-wireless-security.key-mgmt", "wpa-psk",
                    "802-11-wireless-security.psk", store.hotspotPassword];
        return ["nmcli", "connection", "add", "type", "wifi", "ifname", store.iface, "con-name", store.hotspotId,
                "autoconnect", "no", "802-11-wireless.ssid", name, "802-11-wireless.mode", "ap",
                "802-11-wireless-security.key-mgmt", "wpa-psk", "802-11-wireless-security.psk", store.hotspotPassword,
                "ipv4.method", "shared"];
    }

    function hotspotRef() {
        return store.hotspot.uuid !== "" ? ["uuid", store.hotspot.uuid] : ["id", store.hotspotId];
    }

    // write the name and password, then bring a live hotspot up again so they apply at once
    function storeHotspot(bringUp, done, key) {
        if (store.hotspotPassword.length < 8)
            store.hotspotPassword = store.generatePassword();
        if (store.hotspotName.length === 0)
            store.hotspotName = "Ryoku";
        var steps = [store.hotspotArgv()];
        if (bringUp)
            steps.push(["nmcli", "-w", "45", "connection", "up"].concat(store.hotspotRef()));
        store.chain(steps, function (ok, err) {
            store.refresh();
            done(ok, err);
        }, key);
    }

    function setHotspot(on, done, key) {
        if (on) {
            store.storeHotspot(true, done, key);
            return;
        }
        store.run(["nmcli", "connection", "down"].concat(store.hotspotRef()), function (ok, err) {
            store.refresh();
            done(ok, err);
        }, "", key);
    }

    // one command, done(ok, error, out, code) once even if it is missing (code -1); input, a line,
    // goes to stdin; a store key lets cancel(key) stop it
    function run(argv, done, input, key) {
        var p;
        var finish = function (ok, out, err, code, late) {
            if (key)
                store.track(key, p, false);
            // pkexec ends 126 when its prompt is closed and 127 when it is refused; a command past its
            // limit is NetworkManager not answering, not something to name by its program
            var why = ok ? "" : late && argv[0] === "nmcli" ? I18n.tr("NetworkManager did not answer in time. Try again.")
                : argv[0] !== "nmcli" && (code === 126 || code === 127)
                ? I18n.tr("The password prompt was closed or refused. Retry to ask again.") : store.reason(err, out);
            if (done)
                done(ok, why, out, code);
        };
        if (!input) {
            // NetworkManager answers in seconds; a join waits up to its own -w, privileged helpers wait for the password
            p = Changes.run(argv, finish, { timeout: argv[0] !== "nmcli" ? 0 : argv.indexOf("-w") >= 0 ? 60000 : 20000 });
        } else {
            p = proc.createObject(store, { command: argv, input: input, feeds: true });
            p.finished.connect(function (ok, out, err, code) {
                p.destroy();
                finish(ok, out, err, code);
            });
            p.running = true;
        }
        if (key && p.running)
            store.track(key, p, true);
        return p;
    }

    function chain(steps, done, key) {
        if (steps.length === 0) {
            done(true, "");
            return;
        }
        store.run(steps[0], function (ok, err) {
            if (!ok || store.stopped[key])
                done(false, err);
            else
                store.chain(steps.slice(1), done, key);
        }, "", key);
    }

    // the last meaningful line nmcli or a helper printed
    function reason(err, out) {
        var lines = String(err || out || "").split("\n").map(l => l.trim()).filter(l => l.length > 0);
        var line = lines.length > 0 ? lines[lines.length - 1] : "";
        return line.replace(/^Error:\s*/, "");
    }

    // nmcli -t escapes ':' and '\' inside a field
    function terse(line) {
        var fields = [], cur = "";
        for (var i = 0; i < line.length; i++) {
            var c = line[i];
            if (c === "\\" && i + 1 < line.length && (line[i + 1] === ":" || line[i + 1] === "\\")) {
                cur += line[i + 1];
                i++;
            } else if (c === ":") {
                fields.push(cur);
                cur = "";
            } else {
                cur += c;
            }
        }
        fields.push(cur);
        return fields;
    }

    // read the saved profiles again; done() runs once they are in
    function refresh(done) {
        var finish = function (list) {
            store.applyProfiles(list);
            if (done)
                done();
        };
        store.run(["nmcli", "-t", "-f", "NAME,UUID,TYPE,AUTOCONNECT,TIMESTAMP,ACTIVE", "connection", "show"], function (ok, err, out) {
            if (!ok) {
                store.profilesError = err !== "" ? err : I18n.tr("NetworkManager did not answer.");
                if (done)
                    done();
                return;
            }
            store.profilesError = "";
            var list = [];
            String(out).split("\n").forEach(function (line) {
                var f = store.terse(line);
                if (f.length >= 6 && f[2] === "802-11-wireless")
                    list.push({ uuid: f[1], name: f[0], ssid: f[0], autoconnect: f[3] === "yes",
                                timestamp: parseInt(f[4]) || 0, active: f[5] === "yes", band: "", mode: "" });
            });
            if (list.length === 0) {
                finish([]);
                return;
            }
            var argv = ["nmcli", "-t", "-f", "connection.uuid,802-11-wireless.ssid,802-11-wireless.mode,802-11-wireless.band", "connection", "show"];
            list.forEach(p => argv.push("uuid", p.uuid));
            store.run(argv, function (ok2, err2, out2) {
                var byUuid = {}, cur = null;
                String(out2 || "").split("\n").forEach(function (line) {
                    var at = line.indexOf(":");
                    if (at < 0)
                        return;
                    var key = line.slice(0, at), val = store.terse(line.slice(at + 1)).join(":");
                    if (key === "connection.uuid") {
                        cur = { uuid: val };
                        byUuid[val] = cur;
                    } else if (cur && key === "802-11-wireless.ssid") {
                        cur.ssid = val;
                    } else if (cur && key === "802-11-wireless.mode") {
                        cur.mode = val;
                    } else if (cur && key === "802-11-wireless.band") {
                        cur.band = val;
                    }
                });
                finish(list.map(function (p) {
                    var d = byUuid[p.uuid];
                    return d ? Object.assign({}, p, { ssid: d.ssid || p.name, mode: d.mode || "", band: d.band || "" }) : p;
                }));
            });
        });
        backendRead.running = true;
    }

    function applyProfiles(list) {
        var hs = list.find(p => p.name === store.hotspotId || p.mode === "ap") || null;
        store.hotspot = hs ? { uuid: hs.uuid, exists: true, active: hs.active } : { uuid: "", exists: false, active: false };
        store.profiles = list.filter(p => p !== hs && p.mode !== "ap")
            .sort((a, b) => (b.active - a.active) || (b.timestamp - a.timestamp));
        store.profilesRead = true;
        Changes.touch();
        if (hs)
            secretsRead.command = ["nmcli", "-t", "-s", "-g", "802-11-wireless.ssid,802-11-wireless-security.psk", "connection", "show", "uuid", hs.uuid];
        if (hs && !secretsRead.running)
            secretsRead.running = true;
    }

    property Process backendRead: Process {
        command: ["ryoku-host", "network", "wifi-backend", "get"]
        stdout: StdioCollector {
            onStreamFinished: {
                var v = this.text.trim();
                if (v === "iwd" || v === "wpa_supplicant") {
                    store.backend = v;
                    Changes.touch();
                }
            }
        }
    }

    property Process secretsRead: Process {
        stdout: StdioCollector {
            onStreamFinished: {
                var lines = this.text.split("\n");
                if (lines.length >= 1 && lines[0].length > 0)
                    store.hotspotName = lines[0];
                if (lines.length >= 2 && lines[1].length > 0)
                    store.hotspotPassword = lines[1];
                Changes.touch();
            }
        }
    }

    property Timer settle: Timer {
        interval: 1500
        onTriggered: store.refresh()
    }

    property Component proc: Component {
        Process {
            id: p
            property string input: ""
            property bool feeds: false
            property bool didStart: false
            signal finished(bool ok, string out, string err, int code)
            stdinEnabled: p.feeds
            stdout: StdioCollector { id: cout }
            stderr: StdioCollector { id: cerr }
            onStarted: {
                p.didStart = true;
                // stdin closes after the line: nmcli --ask asks again for a refused password and,
                // with stdin still open, waited out its whole timeout instead of failing
                if (p.feeds) {
                    p.write(p.input + "\n");
                    p.input = "";
                    p.stdinEnabled = false;
                }
            }
            onExited: (code, status) => p.finished(code === 0, cout.text, cerr.text, code)
            // a program that cannot start reports no exit, only that it stopped running
            onRunningChanged: {
                if (!p.running && !p.didStart)
                    p.finished(false, "", Changes.missingText(String(p.command[0])), -1);
            }
        }
    }

    property Connections radio: Connections {
        target: Networking
        function onWifiEnabledChanged() { Changes.touch(); }
    }

    // a hotspot started or stopped elsewhere shows up as the radio's mode
    property Connections mode: Connections {
        target: store.wifiDevice
        ignoreUnknownSignals: true
        function onModeChanged() { store.refresh(); }
    }

    Component.onCompleted: {
        Changes.registerStore("wifi", store);
        store.refresh();
    }
}
