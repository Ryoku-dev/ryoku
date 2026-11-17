pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "lib/displays.js" as Displays

// owns screen vibrance (nvibrant) + external monitor brightness (ddcutil) for
// the mixer. persisted vibrance % = source of truth: loaded and pushed once at
// startup so the tint survives a reboot, every later set hits nvibrant AND
// writes the state file back. ddc monitors come from `ddcutil detect` (one
// fader each). setvcp/getvcp wire format lives here so every caller agrees.
Singleton {
    id: root

    readonly property string stateFile: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku/nvibrant-value"

    property int vibrance: 40

    // ddc monitors from `ddcutil detect`: [{ bus, label }]. label = drm connector,
    // else just the i2c bus number.
    property var ddcMonitors: []
    property var probeOwners: []
    readonly property bool probesWanted: root.probeOwners.length > 0
    // `ddcutil detect` walks every i2c bus (28 on a hybrid laptop, 11.5s
    // measured) and i2c traffic on the display controller is felt as the whole
    // session stalling, so it runs once per connector set and only when sysfs
    // says an external display is attached.
    property bool displaysProbed: false
    // A connector change that landed mid-probe: the running answer is stale.
    property bool probeStale: false
    property string connectorKey: ""

    // load saved vibrance % and apply once -> tint survives reboot. singletons
    // init lazily, so something at startup has to actually touch this for the
    // restore to fire.
    function restore() {
        var raw = vibState.text();
        var v = parseInt((raw || "40").trim());
        root.vibrance = isNaN(v) ? 40 : v;
        if (raw && raw.trim().length)
            applyVibrance(root.vibrance);
    }

    // set vibrance to `pct`%: push to nvibrant, save to state. `vibrance` tracks
    // the last value.
    function setVibrance(pct) {
        root.vibrance = Math.round(pct);
        applyVibrance(pct);
        saveVibrance(pct);
    }

    function applyVibrance(pct) {
        var raw = Math.round(Math.max(0, Math.min(100, pct)) * 1023 / 100);
        Quickshell.execDetached(["nvibrant", String(raw), "0", String(raw)]);
    }

    function saveVibrance(pct) {
        Quickshell.execDetached(["sh", "-c",
            'mkdir -p "$(dirname "$1")" && printf "%s\n" "$2" > "$1"',
            "_", root.stateFile, String(Math.round(pct))]);
    }

    function startProbes(owner) {
        if (root.probeOwners.indexOf(owner) < 0)
            root.probeOwners = root.probeOwners.concat([owner]);
        // An open re-reads live values only; the monitor set comes from the cache.
        root.probeDisplays();
        root.readBacklight();
    }

    function stopProbes(owner) {
        root.probeOwners = root.probeOwners.filter(candidate => candidate !== owner);
        if (!root.probesWanted) {
            blRead.running = false;
            root.forgetLevels();
        }
        // ddcDetect keeps running: killing the walk left the answer unknown, so
        // the next open started it again, and every open after that.
    }

    // Ask sysfs before ddcutil: connector status is a file read.
    function probeDisplays() {
        if (root.displaysProbed || connectorProbe.running || ddcDetect.running)
            return;
        connectorProbe.running = true;
    }

    // A connector coming or going is the one event that changes the answer.
    // Nobody watching means the next open probes; nothing walks i2c meanwhile.
    function invalidateDisplays() {
        if (connectorProbe.running || ddcDetect.running) {
            root.probeStale = true;
            return;
        }
        root.displaysProbed = false;
        if (root.probesWanted)
            root.probeDisplays();
    }

    function finishProbe() {
        root.displaysProbed = true;
        if (root.probeStale) {
            root.probeStale = false;
            Qt.callLater(root.invalidateDisplays);
        }
    }

    function detect() {
        if (ddcDetect.running)
            return;
        ddcDetect.running = true;
    }

    // Last level the monitor on each bus is known to hold (read or written),
    // keyed by bus. Cleared when nobody has a brightness surface open so a
    // change made with the monitor's own buttons is never mistaken for it.
    property var ddcLevels: ({})
    // Buses whose first read has finished, whether or not the monitor answered.
    property var ddcSettled: ({})
    property var ddcWrites: ({})
    property var ddcReads: ({})
    property bool ddcBusy: false

    // One ddcutil at a time: overlapping i2c transactions on a bus fail the
    // flock and land out of order. The latest value per bus wins, and it is
    // written once the one in flight is done.
    function setBrightness(bus, pct) {
        var key = String(bus);
        var level = Math.max(0, Math.min(100, Math.round(pct)));
        if (root.ddcLevels[key] === level)
            return;
        root.noteLevel(key, level);
        root.ddcWrites[key] = level;
        root.pumpDdc();
    }

    function readBrightness(bus) {
        var key = String(bus);
        if (key === "")
            return;
        root.ddcReads[key] = true;
        root.pumpDdc();
    }

    function noteLevel(bus, level) {
        var next = Object.assign({}, root.ddcLevels);
        next[bus] = level;
        root.ddcLevels = next;
    }

    function forgetLevels() {
        root.ddcLevels = ({});
        root.ddcSettled = ({});
    }

    function pumpDdc() {
        if (root.ddcBusy)
            return;
        var key;
        for (key in root.ddcWrites) {
            var level = root.ddcWrites[key];
            delete root.ddcWrites[key];
            root.runDdc(key, true, level);
            return;
        }
        for (key in root.ddcReads) {
            delete root.ddcReads[key];
            root.runDdc(key, false, 0);
            return;
        }
    }

    function runDdc(bus, write, level) {
        root.ddcBusy = true;
        ddcJob.createObject(root, { bus: bus, write: write, level: level });
    }

    function ddcDone(bus, write, ok, text) {
        if (write) {
            if (!ok) {
                var next = Object.assign({}, root.ddcLevels);
                delete next[bus];
                root.ddcLevels = next;
                if (root.ddcWrites[bus] === undefined)
                    root.ddcReads[bus] = true;
            }
        } else {
            var seen = Object.assign({}, root.ddcSettled);
            seen[bus] = true;
            root.ddcSettled = seen;
            var value = root.parseBrightness(text);
            if (ok && value >= 0 && root.ddcWrites[bus] === undefined)
                root.noteLevel(bus, value);
        }
        root.ddcBusy = false;
        root.pumpDdc();
    }

    Component {
        id: ddcJob

        Process {
            id: job

            property string bus: ""
            property bool write: false
            property int level: 0
            property int code: -2
            property bool streamed: false
            property bool settled: false
            property string out: ""
            // a process that never started reports neither exit nor stream
            property Timer giveUp: Timer {
                interval: 4000
                running: true
                onTriggered: job.settle()
            }

            command: job.write
                ? ["timeout", "3", "ddcutil", "setvcp", "10", String(job.level), "--bus", job.bus, "--noverify"]
                : ["timeout", "3", "ddcutil", "getvcp", "10", "--brief", "--bus", job.bus]
            running: true

            function settle() {
                if (job.settled)
                    return;
                job.settled = true;
                job.giveUp.stop();
                root.ddcDone(job.bus, job.write, job.code === 0, job.out);
                job.destroy(1000);
            }

            function finishIfDone() {
                if (job.code !== -2 && job.streamed)
                    job.settle();
            }

            stdout: StdioCollector {
                onStreamFinished: {
                    job.out = this.text;
                    job.streamed = true;
                    job.finishIfDone();
                }
            }
            onExited: exitCode => {
                job.code = exitCode;
                job.finishIfDone();
            }
        }
    }

    // pull current brightness % out of a `ddcutil getvcp --brief` line. -1 = none.
    function parseBrightness(text) {
        var m = text.match(/C\s+(\d+)\s+/);
        return m ? parseInt(m[1], 10) : -1;
    }

    // --- internal backlight (brightnessctl): the laptop panel, distinct from
    // the external ddc monitors above. backlightPct stays -1 until the first
    // read, and on a machine with no backlight device the reader never sets it,
    // so backlightAvailable stays false and the fader hides.
    property int backlightPct: -1
    readonly property bool backlightAvailable: root.backlightPct >= 0

    function readBacklight() {
        if (!root.probesWanted)
            return;
        blRead.running = false;
        blRead.running = true;
    }

    // set the panel brightness to `pct`%, floored at 1 so it never blacks out.
    // tracks the value optimistically so the fader does not snap back while the
    // poll catches up.
    function setBacklight(pct) {
        var p = Math.max(1, Math.min(100, Math.round(pct)));
        root.backlightPct = p;
        // Resolve the panel through ryoku-hw-backlight, the same selector the
        // media keys and the OSD daemon use, so the fader drives the connected
        // internal panel. brightnessctl's own default pick lands on a phantom
        // nvidia_*/acpi_video device on a laptop with a discrete GPU, and the
        // write then succeeds against a device that changes nothing (#221).
        // With no backlight at all the selector prints nothing, and the class
        // keeps brightnessctl off its fallback pick, a keyboard LED.
        Quickshell.execDetached(["bash", "-c",
            "BL=$(ryoku-hw-backlight 2>/dev/null); brightnessctl -c backlight ${BL:+-d \"$BL\"} set \"$1%\" >/dev/null 2>&1",
            "_", String(p)]);
    }

    // eDP/LVDS is the backlight's business and Writeback is not an output, so
    // neither is a reason to walk i2c.
    Process {
        id: connectorProbe
        command: ["sh", "-c",
            "for c in /sys/class/drm/*-*; do "
            + "case \"${c##*/}\" in *eDP*|*LVDS*|*Writeback*) continue ;; esac; "
            + "[ \"$(cat \"$c/status\" 2>/dev/null)\" = connected ] && { echo external; exit 0; }; "
            + "done; echo panel-only"]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                if (this.text.indexOf("external") >= 0) {
                    root.detect();
                    return;
                }
                root.ddcMonitors = [];
                root.finishProbe();
            }
        }
    }

    Process {
        id: ddcDetect
        command: ["ddcutil", "detect", "--brief"]
        running: false
        // Answered on exit, not on the last byte: a missing ddcutil or a refused
        // bus must still count, or every open asks again.
        onExited: root.finishProbe()
        stdout: StdioCollector {
            onStreamFinished: {
                var mons = [];
                var blocks = this.text.split(/\bDisplay \d+/);
                // blocks[0] is the invalid-display preamble (the internal panel
                // among them): a fader there writes to a bus nothing answers.
                for (var i = 1; i < blocks.length; i++) {
                    var bus = /I2C bus:\s+\/dev\/i2c-(\d+)/.exec(blocks[i]);
                    var conn = /DRM connector:\s+card\d+-(\S+)/.exec(blocks[i]);
                    if (bus)
                        mons.push({ bus: bus[1], label: conn ? conn[1] : "BUS " + bus[1] });
                }
                root.ddcMonitors = mons;
            }
        }
    }

    Process {
        id: blRead
        command: ["bash", "-c", "BL=$(ryoku-hw-backlight 2>/dev/null); brightnessctl -c backlight ${BL:+-d \"$BL\"} -m 2>/dev/null | awk -F, 'NR==1{print substr($4,1,length($4)-1)}'"]
        running: false
        stdout: StdioCollector {
            onStreamFinished: {
                if (!root.probesWanted)
                    return;
                var v = parseInt(this.text.trim());
                if (!isNaN(v))
                    root.backlightPct = v;
            }
        }
    }

    // Hotplug: a display plugged in after login gets its fader without a restart.
    Connections {
        target: Wm
        function onOutputsChanged() {
            var key = Displays.connectorSignature(Wm.outputs);
            if (key === root.connectorKey)
                return;
            root.connectorKey = key;
            Qt.callLater(root.invalidateDisplays);
        }
    }

    FileView {
        id: vibState
        path: root.stateFile
        blockLoading: true
        printErrors: false
    }
}
