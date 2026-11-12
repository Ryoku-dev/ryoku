import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// the saved setup in use as a Changes store: writing a name applies that
// setup, writing "" puts back the one in use before it
QtObject {
    id: root

    property string applied: ""
    property bool busy: false
    // the last run found the wallpaper service down: the next one starts it first
    property bool startService: false
    readonly property string startScript: "systemctl --user reset-failed ryogami.service; systemctl --user start ryogami.service || exit 1; "
        + "for i in $(seq 50); do ryogami settings get theme.mode >/dev/null 2>&1 && exec \"$@\"; sleep 0.1; done; exec \"$@\""

    function read(key) {
        return root.applied;
    }

    function write(key, value, done) {
        if (root.busy) {
            done(false, I18n.tr("Another setup is still being applied. Try again when it finishes."));
            return;
        }
        root.busy = true;
        var target = String(value || "");
        var cmd = target !== "" ? ["ryoku-hub", "rice", "apply", target, "all"] : ["ryoku-hub", "rice", "restore", "previous"];
        var argv = root.startService ? ["sh", "-c", root.startScript, "sh"].concat(cmd) : cmd;
        root.startService = false;
        Changes.run(argv, function (ok, out, err, code) {
            root.busy = false;
            if (ok) {
                root.applied = target;
                Changes.reloadDesktop();
                Changes.touch();
                done(true, "");
                return;
            }
            var why = String(err || "").trim().split("\n").pop().replace(/^ryoku-hub:\s*/, "").replace(/\.+$/, "");
            // code -1 is a missing program, or an apply stopped after 3 minutes (installing shells can take a while)
            if (code === -1) {
                done(false, err === Changes.missingText(argv[0])
                    ? I18n.tr("The setup could not be applied because a Ryoku tool is missing. Update Ryoku, then try again.")
                    : target !== "" ? I18n.tr("Applying the setup took too long, so Settings stopped it. Part of it may already be in place; press Retry to apply it again.")
                    : I18n.tr("Putting your previous setup back took too long, so Settings stopped it. Part of it may already be back; press Retry to finish."));
                return;
            }
            // colours go through the wallpaper service, which must be running
            if (/daemon not reachable/.test(why)) {
                root.startService = true;
                done(false, target !== ""
                    ? I18n.tr("The colours could not be set because the wallpaper service is not running. Press Retry to start it and finish.")
                    : I18n.tr("Your previous colours could not be put back because the wallpaper service is not running. Press Retry to start it and finish."));
            } else {
                done(false, (target !== "" ? I18n.tr("The setup did not fully apply: %1. Press Retry.")
                                           : I18n.tr("Your previous setup did not fully come back: %1. Press Retry."))
                    .replace("%1", why || I18n.tr("it stopped early")));
            }
        }, { timeout: 180000 });
    }

    Component.onCompleted: Changes.registerStore("setup", root)
}
