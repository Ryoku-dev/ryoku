import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// the system time zone as a Changes store: read once, written through the host
// time seam, then the shell restarts so its clocks pick the zone up
QtObject {
    id: root

    property string zone: ""
    property bool known: false
    property bool busy: false
    // the last read ended without a zone (refused, missing, hung or empty)
    property bool readFailed: false
    property bool reading: false

    function read(key) {
        return root.known ? root.zone : undefined;
    }

    function write(key, value, done) {
        if (root.busy) {
            done(false, I18n.tr("The time zone is still changing. Try again when it finishes."));
            return;
        }
        root.busy = true;
        var argv = ["ryoku-host", "time", "set-zone", String(value)];
        Changes.run(argv, function (ok, out, err, code) {
            root.busy = false;
            if (ok) {
                root.readZone();
                restartProc.running = true;
            }
            // the backend's words (timedatectl, pkexec) are not shown; they only pick the message
            var denied = /not authori|access denied|authenticat|permission/i.test(String(err || ""));
            // code -1 is a missing program, or a write stopped after 15 s
            done(ok, ok ? "" : code === -1
                ? (err === Changes.missingText(argv[0])
                    ? I18n.tr("The time zone could not be changed because a Ryoku system tool is missing. Update Ryoku, then try again.")
                    : I18n.tr("The system took too long to change the time zone, so Settings stopped waiting. Press Retry."))
                : code === 2 ? I18n.tr("The system does not know this time zone. Choose another one on the map.")
                : denied ? I18n.tr("You were not allowed to change the time zone. Press Retry and enter your password when asked.")
                : I18n.tr("The system did not take the new time zone. Press Retry, and if it still fails, update Ryoku from Updates."));
        }, { timeout: 15000 });
    }

    // a read that fails or hangs ends after 10 s, so the row says so instead of waiting
    function readZone() {
        if (root.reading)
            return;
        root.reading = true;
        Changes.run(["ryoku-host", "time", "zone"], function (ok, out) {
            root.reading = false;
            var z = ok ? out.trim() : "";
            root.readFailed = z === "";
            if (z !== "") {
                root.zone = z;
                root.known = true;
            }
            Changes.touch();
        }, { timeout: 10000 });
    }

    // a running process keeps the zone it started with, so the shell's clocks need a fresh start
    property Process restartProc: Process {
        command: ["ryoku-host", "svc", "restart", "ryoku-shell"]
        onExited: (code, status) => {
            if (code === 4)
                Bus.run("reload");
        }
    }

    Component.onCompleted: {
        Changes.registerStore("timezone", root);
        root.readZone();
    }
}
