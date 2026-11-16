import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

// the system card's sharing: Copy and Save draw a still card at twice its size, its code
// rulers carrying the design in its pixels, and `ryoku-hub profile card` adds the whole design
// for a file sent as it is. Open and Paste take a design back out of a card picture (from the
// pixels when a chat app re-encoded it), the clipboard or a .ryoprofile. `message` and
// `error` say how the last one went, and retry() runs it again
Item {
    id: root

    required property var hub
    property string pageId: ""

    property string message: ""
    property string error: ""
    // the job running now ("copy", "save", "use", "paste"), or ""
    property string job: ""
    property string last: ""
    property string arg: ""
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
            var ok = !!stillLoader.item && SysInfo.loaded === true && !SysInfo.loading && CardCode.ready;
            if (!ok && ++ready.tries < 40)
                return;
            ready.stop();
            if (!CardCode.ready) {
                root.job = "";
                root.error = I18n.tr("The card's code could not be made. Update Ryoku, then try again.");
                return;
            }
            root.draw(root.job);
        }
    }
    Loader {
        id: stillLoader
        active: false
        sourceComponent: TradingCard {}
    }

    function draw(job) {
        var rendered = (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-card-render.png";
        var name = String(SysInfo.sysUser || "card").replace(/[^A-Za-z0-9._-]/g, "") || "card";
        var dst = job === "copy" ? "--copy" : Paths.picturesDir + "/ryoku-card-" + name + "-" + Qt.formatDateTime(new Date(), "yyyyMMdd-HHmmss") + ".png";
        var card = stillLoader.item;
        var ok = card.grabToImage(function (result) {
            if (!result.saveToFile(rendered)) {
                root.job = "";
                root.error = I18n.tr("The card could not be drawn into a picture. Try again.");
                return;
            }
            root.run(job, dst, rendered);
        }, Qt.size(card.designWidth * 2, card.designHeight * 2));
        if (!ok) {
            root.job = "";
            root.error = I18n.tr("The card could not be drawn into a picture. Try again.");
        }
    }

    // someone's card: a picture or .ryoprofile ("use", picked in Open), or the clipboard ("paste")
    function open() {
        if (root.job === "")
            chooser.open();
    }
    function paste() {
        if (root.job === "")
            root.use("paste", "");
    }
    function use(job, file) {
        // a picture the card tool cannot read (WebP, AVIF, ...) is turned into a PNG first
        if (job === "use" && !/\.(png|jpe?g|gif|ryoprofile)$/i.test(file)) {
            convert.pending = file;
            convert.source = "file://" + file;
            return;
        }
        root.hub.confirm({
            title: I18n.tr("Use this card's design?"),
            text: I18n.tr("Its figure, words and choices replace your card's. To keep yours, copy or save it first."),
            actionText: I18n.tr("Use it"),
            destructive: false
        }, () => root.run(job, file, ""));
    }

    function reset() {
        root.hub.confirm({
            title: I18n.tr("Back to the stock card?"),
            text: I18n.tr("Your figure, words and choices are forgotten. Undo is offered right after."),
            actionText: I18n.tr("Reset the card"),
            destructive: true
        }, () => Changes.set(root.cardAll, ({}), I18n.tr("System card"), root.pageId, I18n.tr("The card is back to stock")));
    }

    // a refused job, in plain words with its next step; the tool's own words are not shown
    function refusal(job, err) {
        var e = String(err || "");
        var use = job === "use" || job === "paste";
        if (/no space left/i.test(e))
            return use ? I18n.tr("The disk is full, so the design could not be used. Free some space, then press Retry.")
                       : I18n.tr("The disk is full, so the card could not be saved. Free some space, then press Retry.");
        if (job === "paste" && /no picture/i.test(e))
            return I18n.tr("The clipboard holds no picture. Copy a Ryoku card first, then press Paste.");
        if (use && /card code|not a Ryoku card|stripped/i.test(e))
            return I18n.tr("Settings could not read a card in that picture. Use the whole card picture as it was shared, not a crop or a screenshot of part of it.");
        if (use && /could not be read/i.test(e))
            return I18n.tr("That picture could not be read. Choose another with Open.");
        if (use && /ryoprofile|hero|not a PNG|cut short|no end|invalid character|unexpected end|cannot unmarshal/i.test(e))
            return I18n.tr("That file is not a card, or it is damaged. Choose a card picture with Open.");
        if (job === "copy" && /wl-copy/i.test(e))
            return I18n.tr("The card could not be put on the clipboard. Save it instead, or press Retry.");
        if (/permission denied|read-only file system|operation not permitted/i.test(e))
            return use ? I18n.tr("Settings is not allowed to read that file or save your card. Check the file's permissions, then press Retry, or choose another with Open.")
                       : I18n.tr("Settings is not allowed to write in %1. Check the folder's permissions, then press Retry.").replace("%1", root.tilde(Paths.picturesDir));
        if (/no such file|not a directory/i.test(e))
            return use ? I18n.tr("That file is no longer there. Choose another with Open.")
                       : I18n.tr("%1 is not there. Create it, then press Retry.").replace("%1", root.tilde(Paths.picturesDir));
        return use ? I18n.tr("That design could not be used. Press Retry, or choose another with Open.")
                   : I18n.tr("The card could not be made. Press Retry.");
    }

    function run(job, arg, rendered) {
        root.error = "";
        root.message = "";
        root.last = job;
        root.arg = arg;
        root.job = job;
        var argv = job === "use" ? ["ryoku-hub", "profile", "import", arg]
            : job === "paste" ? ["ryoku-hub", "profile", "import", "--clipboard"]
            : ["ryoku-hub", "profile", "card", rendered, arg];
        Changes.run(argv, function (ok, out, err, code) {
            root.job = "";
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
            if (job === "copy")
                root.message = I18n.tr("Copied. Paste it in a chat or anywhere; Ryoku Settings opens it as a card.");
            else if (job === "save")
                root.message = I18n.tr("Saved to %1").replace("%1", root.tilde(arg));
            else
                root.message = I18n.tr("Your card uses the shared design");
        }, { timeout: 30000 });
    }

    function retry() {
        if (root.last === "copy" || root.last === "save")
            root.take(root.last);
        else if (root.last !== "")
            root.run(root.last, root.arg, "");
    }

    Image {
        id: convert
        property string pending: ""
        asynchronous: true
        onStatusChanged: {
            if (convert.pending === "" || convert.status === Image.Loading || convert.status === Image.Null)
                return;
            var file = convert.pending;
            convert.pending = "";
            if (convert.status !== Image.Ready) {
                root.error = I18n.tr("That picture could not be read. Choose another with Open.");
                return;
            }
            var out = (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-card-open.png";
            convert.grabToImage(function (result) {
                if (result.saveToFile(out))
                    root.use("use", out);
                else
                    root.error = I18n.tr("That picture could not be read. Choose another with Open.");
            }, Qt.size(convert.sourceSize.width, convert.sourceSize.height));
        }
        width: sourceSize.width
        height: sourceSize.height
    }

    // a card someone shared is a picture, so it is chosen by looking at it
    ImageChooser {
        id: chooser
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Open a card")
        extraFilters: ["*.ryoprofile"]
        onAccepted: root.use("use", chooser.localPath(chooser.selectedFile))
    }
}
