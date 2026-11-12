import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// the window manager's animation style as a Changes store: a file the backend
// writes, after which it reloads the window manager
QtObject {
    id: store

    property var preset: undefined

    function read(key) {
        return key === "preset" ? store.preset : undefined;
    }

    function write(key, v, done) {
        if (key !== "preset") {
            done(false, "");
            return;
        }
        var before = store.preset;
        store.preset = String(v);
        Changes.touch();
        // a reload that hangs ends as a failure after 10 s, so the picker never waits forever
        Changes.run(["ryoku-hub", "desktop", "anim-preset", String(v)], function (ok, out, err, code) {
            // the control shows what really applied, even when the re-read cannot run
            if (!ok) {
                store.preset = before;
                Changes.touch();
            }
            done(ok, ok ? "" : code === -1 ? err : I18n.tr("The animation style did not apply. Try again, or pick another style."));
            store.reread();
        }, { timeout: 10000 });
    }

    function reread() {
        store.getProc.running = false;
        store.getProc.running = true;
    }

    property Process getProc: Process {
        command: ["ryoku-hub", "desktop", "anim-preset"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    store.preset = String(JSON.parse(this.text).preset || "ryoku");
                } catch (e) {
                    store.preset = undefined;
                }
                Changes.touch();
            }
        }
    }

    Component.onCompleted: Changes.registerStore("animations", store)
}
