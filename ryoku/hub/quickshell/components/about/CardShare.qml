import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

// Copy and Save draw only the owner's card. Friends collect the picture in their binder.
Item {
    id: root

    required property var hub
    property string pageId: ""

    property string message: ""
    property string error: ""
    // the job running now ("copy" or "save"), or ""
    property string job: ""
    property string last: ""
    property string syncPath: ""
    signal captured(string path)
    signal captureFailed(string reason)

    function fail(reason) {
        var syncing = root.job === "sync";
        root.job = "";
        root.error = reason;
        stillLoader.active = false;
        if (syncing) {
            if (root.syncPath !== "")
                Changes.run(["rm", "-f", "--", root.syncPath]);
            root.syncPath = "";
            root.captureFailed(reason);
        }
    }
    readonly property var cardAll: ({ store: "card", key: "*" })

    // drawn, so the still card can be grabbed, but out of sight
    width: 0
    height: 0
    clip: true

    function tilde(p) {
        var home = Quickshell.env("HOME") || "";
        return home.length > 1 && String(p).indexOf(home) === 0 ? "~" + String(p).slice(home.length) : String(p);
    }

    // the card is drawn once the machine's full read-out and the card's code are in
    function take(job) {
        if (root.job !== "")
            return;
        root.error = "";
        root.message = "";
        root.last = job;
        root.job = job;
        if (job === "sync") {
            Changes.run(["mktemp", "--suffix=.png", (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-card-sync-XXXXXXXX"], function(ok, out) {
                if (!ok) {
                    root.fail(I18n.tr("A temporary card picture could not be created. Check the free space and try again."));
                    return;
                }
                root.syncPath = out.trim();
                root.prepare();
            });
        } else {
            root.prepare();
        }
    }
    function prepare() {
        stillLoader.active = true;
        if (!SysInfo.loaded && !SysInfo.loading)
            SysInfo.load();
        ready.tries = 0;
        ready.restart();
    }
    Timer {
        id: ready
        property int tries: 0
        interval: 150
        repeat: true
        onTriggered: {
            var ok = !!stillLoader.item && stillLoader.item.imagesReady && SysInfo.loaded === true && !SysInfo.loading && CardCode.ready;
            if (!ok && ++ready.tries < 40)
                return;
            ready.stop();
            if (!ok) {
                root.fail(I18n.tr("The card's picture or code could not be loaded. Update Ryoku, then try again."));
                return;
            }
            root.draw(root.job);
        }
    }
    Loader {
        id: stillLoader
        active: false
        sourceComponent: TradingCard {
            design: root.job === "sync" ? Badge.design : Badge.ownDesign
            sealed: root.job !== "sync"
        }
    }

    function draw(job) {
        var rendered = job === "sync" ? root.syncPath : (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-card-render.png";
        var name = String(SysInfo.sysUser || "card").replace(/[^A-Za-z0-9._-]/g, "") || "card";
        var dst = job === "copy" ? "--copy" : Paths.picturesDir + "/ryoku-card-" + name + "-" + Qt.formatDateTime(new Date(), "yyyyMMdd-HHmmss") + ".png";
        var card = stillLoader.item;
        var ok = card.grabToImage(function (result) {
            if (!result.saveToFile(rendered)) {
                root.fail(I18n.tr("The card could not be drawn into a picture. Try again."));
                return;
            }
            if (job === "sync") {
                root.job = "";
                root.syncPath = "";
                root.captured(rendered);
                stillLoader.active = false;
                return;
            }
            root.run(job, dst, rendered);
        }, Qt.size(card.designWidth * 2, card.designHeight * 2));
        if (!ok) {
            root.fail(I18n.tr("The card could not be drawn into a picture. Try again."));
        }
    }


    function reset() {
        root.hub.confirm({
            title: I18n.tr("Back to the stock card?"),
            text: I18n.tr("Your figure, words and choices are forgotten. Undo is offered right after."),
            actionText: I18n.tr("Reset the card"),
            destructive: true
        }, () => Changes.set(root.cardAll, ({}), I18n.tr("System card"), root.pageId, I18n.tr("The card is back to stock")));
    }

    function refusal(job, err) {
        var e = String(err || "");
        if (/no space left/i.test(e))
            return I18n.tr("The disk is full, so the card could not be saved. Free some space, then press Retry.");
        if (job === "copy" && /wl-copy/i.test(e))
            return I18n.tr("The card could not be put on the clipboard. Save it instead, or press Retry.");
        if (/permission denied|read-only file system|operation not permitted/i.test(e))
            return I18n.tr("Settings is not allowed to write in %1. Check the folder's permissions, then press Retry.").replace("%1", root.tilde(Paths.picturesDir));
        if (/no such file|not a directory/i.test(e))
            return I18n.tr("%1 is not there. Create it, then press Retry.").replace("%1", root.tilde(Paths.picturesDir));
        return I18n.tr("The card could not be made. Press Retry.");
    }

    function run(job, arg, rendered) {
        root.error = "";
        root.message = "";
        root.last = job;
        root.job = job;
        var argv = ["ryoku-hub", "profile", "card", rendered, arg];
        Changes.run(argv, function (ok, out, err, code) {
            root.job = "";
            stillLoader.active = false;
            if (code === -1) {
                root.error = err === Changes.missingText(argv[0])
                    ? I18n.tr("The card tool is missing from this Ryoku install. Update Ryoku, then try again.")
                    : I18n.tr("That took too long, so Settings stopped waiting. Try again.");
                return;
            }
            if (!ok) {
                root.error = root.refusal(job, err);
                return;
            }
            Progression.award("showcase");
            if (job === "copy")
                root.message = I18n.tr("Copied. Send the picture to a friend; they can paste it into their binder.");
            else if (job === "save")
                root.message = I18n.tr("Saved to %1").replace("%1", root.tilde(arg));
        }, { timeout: 30000 });
    }

    function retry() {
        if (root.last === "copy" || root.last === "save")
            root.take(root.last);
    }

}
