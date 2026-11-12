import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/terminal"
import "../schema/terminal.js" as Schema

// the login shell, the prompt and the greeting a new terminal shows
Item {
    id: root

    required property var hub
    readonly property string pageId: "terminal"

    readonly property var store: Changes.stores["terminal"] || null
    readonly property var ff: root.store ? root.store.ffModel : null
    readonly property var logoBind: ({ store: "terminal", key: "ff.logo" })
    readonly property var linesBind: ({ store: "terminal", key: "ff.rows" })
    readonly property bool locked: !!(root.ff && root.ff.renderer)

    // the action running now, by row id, and what went wrong last
    property string busyId: ""
    property string actionError: ""
    property var retryAction: null
    property string pictureError: ""
    property var retryPictureAction: null
    property string textArtError: ""
    property real shellWait: 0
    // the folder each file dialog last chose from, so reopening starts there
    property string pictureFolder: ""
    property string textArtFolder: ""

    readonly property string picsRoot: {
        var home = Paths.home, dir = Paths.picturesDir;
        return home.length > 0 && dir.indexOf(home + "/") === 0 ? "~" + dir.slice(home.length) : dir;
    }
    readonly property var brandArt: [
        { name: "Ryoku", path: "~/.local/share/ryoku/assets/brand/fastfetch-emblem.png" },
        { name: "Ryoku mark", path: "~/.local/share/ryoku/assets/brand/logo-mark.png" },
        { name: "Laocoon", path: root.picsRoot + "/ryodecors/laocoon.png" },
        { name: "David", path: root.picsRoot + "/ryodecors/david.png" },
        { name: "Aurelius", path: root.picsRoot + "/ryodecors/aurelius.png" },
        { name: "Hawk", path: root.picsRoot + "/ryodecors/hawk.png" },
        { name: "Katana", path: root.picsRoot + "/ryodecors/katana.png" },
        { name: "Moon", path: root.picsRoot + "/ryodecors/moon.png" },
        { name: "Needle", path: root.picsRoot + "/ryodecors/needle.png" },
        { name: "Microphone", path: root.picsRoot + "/ryodecors/mic.png" },
        { name: "Camera", path: root.picsRoot + "/ryodecors/camera.png" },
        { name: "Rashin", path: root.picsRoot + "/ryodecors/rashin-hero.png" }
    ]
    readonly property var readyArt: root.brandArt.concat((root.store ? root.store.installedArt : []).map(file => {
        var stem = file.replace(/\.png$/i, "");
        var words = stem.replace(/-/g, " ");
        return { name: words.charAt(0).toUpperCase() + words.slice(1), path: root.picsRoot + "/ryoemblems/" + file, id: stem, store: true };
    }))

    readonly property var shellFiles: ({
        fish: "~/.config/fish/user.fish",
        bash: "~/.config/bash/user.bash",
        zsh: "~/.config/zsh/user.zsh"
    })
    readonly property var layoutNames: ({
        "pill": "Pill", "minimal": "Minimal", "two-line": "Two line", "powerline": "Powerline", "lean": "Lean"
    })

    function expand(path) {
        return String(path).charAt(0) === "~" ? Paths.home + String(path).slice(1) : String(path);
    }

    function folderOf(path) {
        var p = root.expand(path);
        var at = p.lastIndexOf("/");
        return at > 0 ? p.slice(0, at) : "";
    }

    // why a chosen file could not be used, in plain words with what to do next
    function fileError(err, fallback) {
        var t = String(err || "");
        var reason = /not found|no such file/i.test(t) ? I18n.tr("It is no longer there.")
            : /permission denied/i.test(t) ? I18n.tr("It cannot be read.")
            : "";
        if (reason.length === 0)
            return root.store.cleanError(err, fallback);
        return fallback + " " + reason + " " + I18n.tr("Choose another file.");
    }

    // a schema row with what only this machine knows filled in, or null to leave it out
    function prepared(row) {
        var s = root.store;
        if (!s)
            return null;
        var out = Object.assign({}, row, { host: root });
        switch (row.id) {
        case "login-shell":
            if (!s.shellLoaded)
                return null;
            out.options = s.shell.choices.filter(c => c.installed === true).map(c => ({ label: c.label, value: c.key }));
            return out;
        case "zsh-prompt":
            return s.shellLoaded && s.shell.choices.some(c => c.key === "zsh" && c.installed === true) ? out : null;
        case "own-lines":
            if (!s.shellLoaded || !root.shellFiles[s.shell.current])
                return null;
            out.detail = s.shell.current === "zsh"
                ? I18n.tr("Lines you add to %1 run in every new terminal. Oh My Zsh keeps its files in ~/.config/zsh/oh-my-zsh.").arg(root.shellFiles.zsh)
                : I18n.tr("Lines you add to %1 run in every new terminal.").arg(root.shellFiles[s.shell.current]);
            out.command = ["ryoku-app", "editor", "--", root.expand(root.shellFiles[s.shell.current])];
            return out;
        case "prompt-style":
            if (!s.starshipLoaded)
                return null;
            var order = ["pill", "minimal", "two-line", "powerline", "lean"];
            var ids = order.filter(id => s.layouts.indexOf(id) >= 0).concat(s.layouts.filter(id => order.indexOf(id) < 0));
            out.options = ids.map(id => ({ label: root.layoutNames[id] || id, value: id }));
            out.unavailable = ids.length === 0 ? "No prompt styles were found on this computer." : "";
            return out;
        case "prompt-colours":
        case "prompt-edit-colours":
            return s.starshipLoaded ? out : null;
        case "greeting-file":
            out.command = ["ryoku-app", "editor", "--", s.configPath];
            return s.ffLoaded ? out : null;
        case "greeting-style":
            out.placeholder = "Choose a style";
            out.run = "applyStyle";
            out.unavailable = s.styles.length === 0 ? "No greeting styles installed from RyoStore yet" : "";
            return s.ffLoaded ? out : null;
        case "greeting-remove-style":
            out.placeholder = "Choose a style";
            out.run = "removeStyle";
            return s.ffLoaded && s.styles.length > 0 ? out : null;
        case "greeting-reset":
            out.actionText = "Reset";
            out.actionKind = "dangerLine";
            out.run = "resetGreeting";
            return s.ffLoaded ? out : null;
        case "greeting-open-picture":
            if (!s.ffLoaded || !root.ff || !root.ff.logo.source)
                return null;
            out.command = root.ff.logo.kind === "ascii"
                ? ["ryoku-app", "editor", "--", root.expand(root.ff.logo.source)]
                : ["xdg-open", root.expand(root.ff.logo.source)];
            return out;
        case "greeting-fit":
            out.actionText = "Fit";
            out.run = "autoFit";
            out.unavailable = root.ff && !root.ff.logo.source ? "Choose a picture first" : "";
            return s.ffLoaded ? out : null;
        case "greeting-dither":
            out.unavailable = root.ff && !root.ff.logo.source ? "Choose a picture first" : "";
            return s.ffLoaded ? out : null;
        case "greeting-remove-picture":
            if (!s.ffLoaded || !root.ff || root.ff.logo.kind !== "image" || !/\/fastfetch\/ryoku-logo\./.test(String(root.ff.logo.source || "")))
                return null;
            out.actionText = "Remove";
            out.actionKind = "dangerLine";
            out.run = "removePicture";
            return out;
        }
        if (row.bind && String(row.bind.key).indexOf("ff.") === 0 && !s.ffLoaded)
            return null;
        return out;
    }

    readonly property var rows: {
        Changes.revision;
        return Schema.rows.map(r => r.template === true ? r : root.prepared(r)).filter(r => r !== null);
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: root.rows
    })

    readonly property var controlMap: ({
        "greeting-style": styleControl,
        "greeting-remove-style": styleControl,
        "greeting-reset": actionControl,
        "greeting-fit": actionControl,
        "greeting-remove-picture": actionControl,
        "greeting-text-art": textArtControl
    })

    readonly property var stateLine: {
        var s = root.store;
        if (!s || !s.shellLoaded || !s.starshipLoaded)
            return "";
        var shell = (s.shell.choices.find(c => c.key === s.shell.current) || { label: s.shell.current }).label;
        return I18n.tr("%1 with the %2 prompt").arg(shell).arg(I18n.tr(root.layoutNames[s.starship.layout] || s.starship.layout));
    }

    // a landing waits until everything this page reads has arrived
    readonly property bool complete: !!root.store
        && (root.store.shellLoaded || root.store.shellReadError.length > 0)
        && (root.store.starshipLoaded || root.store.starshipReadError.length > 0)
        && (root.store.ffLoaded || root.store.ffReadError.length > 0)
    property string pendingRow: ""
    onCompleteChanged: {
        if (root.complete && root.pendingRow !== "") {
            var id = root.pendingRow;
            root.pendingRow = "";
            Qt.callLater(page.focusRow, id);
        }
    }

    function focusRow(rowId) {
        if (!root.complete) {
            root.pendingRow = rowId;
            return;
        }
        page.focusRow(rowId);
    }

    // one-off commands: busy while they run, an error with Retry when they fail
    function act(id, argv, failText, after) {
        root.busyId = id;
        root.actionError = "";
        root.store.run(argv, (ok, out, err) => {
            root.busyId = "";
            if (!ok) {
                root.actionError = root.store.cleanError(err, failText);
                root.retryAction = () => root.act(id, argv, failText, after);
                return;
            }
            root.retryAction = null;
            if (after)
                after(out);
        });
    }

    function newLogo(change) {
        return Object.assign({}, root.ff.logo, change);
    }

    function pickArt(path) {
        root.pictureError = "";
        Changes.set(root.logoBind, root.newLogo({ kind: "image", source: path, dither: false }), "Greeting picture", root.pageId);
    }

    function importImage(path) {
        if (!path)
            return;
        root.pictureError = "";
        root.busyId = "greeting-image";
        root.store.run(["ryoku-hub", "fastfetch", "import-logo", path], (ok, out, err) => {
            root.busyId = "";
            var source = out.trim();
            if (!ok || source.length === 0) {
                root.pictureError = root.fileError(err, I18n.tr("That picture could not be used."));
                root.retryPictureAction = () => root.importImage(path);
                return;
            }
            root.pictureFolder = root.folderOf(path);
            root.pickArt(source);
        });
    }

    function retryPicture() {
        if (root.retryPictureAction)
            root.retryPictureAction();
    }

    function importTextArt(path) {
        if (!path)
            return;
        root.textArtError = "";
        root.store.run(["ryoku-hub", "fastfetch", "import-logo", path], (ok, out, err) => {
            var source = out.trim();
            if (!ok || source.length === 0) {
                root.textArtError = root.fileError(err, I18n.tr("That file could not be used."));
                return;
            }
            root.textArtFolder = root.folderOf(path);
            Changes.set(root.logoBind, root.newLogo({ kind: "ascii", source: source, dither: false }), "Text art file", root.pageId);
        });
    }

    function removeStoreArt(art) {
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(I18n.tr(art.name)),
            text: I18n.tr("The picture is deleted from your Pictures folder. You can install it again from RyoStore."),
            actionText: I18n.tr("Remove")
        }, () => {
            var wasCurrent = root.ff && root.ff.logo.source === art.path;
            root.act("greeting-image", ["ryostore", "remove", "fastfetch-emblems", art.id], I18n.tr("The picture was not removed."), () => {
                root.store.artScan.running = true;
                // a missing picture would fall back to the system logo, so the greeting moves to Ryoku's emblem
                if (wasCurrent)
                    root.pickArt(root.brandArt[0].path);
            });
        });
    }

    function removePicture() {
        root.hub.confirm({
            title: I18n.tr("Remove your imported picture?"),
            text: I18n.tr("The copy the greeting uses is deleted and the greeting shows Ryoku's emblem again. Your original file stays."),
            actionText: I18n.tr("Remove")
        }, () => root.act("greeting-remove-picture", ["ryoku-hub", "fastfetch", "remove-logo"],
                          I18n.tr("The picture was not removed."), () => root.store.reloadGreeting()));
    }

    function applyStyle(style) {
        root.hub.confirm({
            title: I18n.tr("Use the %1 greeting?").arg(style.name || style.id),
            text: I18n.tr("It replaces your greeting, its picture, colours and lines. This cannot be undone."),
            actionText: I18n.tr("Use it"),
            destructive: false
        }, () => root.act("greeting-style", ["ryostore", "internal", "apply-fastfetch", style.id],
                          I18n.tr("The style was not applied."), () => root.store.reloadGreeting()));
    }

    function removeStyle(style) {
        root.hub.confirm({
            title: I18n.tr("Remove %1 from your library?").arg(style.name || style.id),
            text: I18n.tr("The style is deleted. A greeting already using it stays as it is."),
            actionText: I18n.tr("Remove")
        }, () => root.act("greeting-remove-style", ["ryostore", "remove", "fastfetch", style.id],
                          I18n.tr("The style was not removed."), () => root.store.reloadGreeting()));
    }

    function resetGreeting() {
        root.hub.confirm({
            title: I18n.tr("Reset the greeting?"),
            text: I18n.tr("Your picture, colours and lines are replaced by a plain greeting: the system logo and five lines. This cannot be undone."),
            actionText: I18n.tr("Reset")
        }, () => root.act("greeting-reset", ["ryoku-hub", "fastfetch", "reset"],
                          I18n.tr("The greeting was not reset."), () => root.store.reloadGreeting()));
    }

    // keep the width and size the height to the picture, in the terminal's own cells
    function autoFit() {
        if (probe.status !== Image.Ready || probe.implicitHeight <= 0 || cells.height <= 0)
            return;
        var aspect = probe.implicitWidth / probe.implicitHeight;
        var w = root.ff.logo.width > 0 ? root.ff.logo.width : 28;
        var h = Math.max(1, Math.min(60, Math.round(w * (cells.averageCharacterWidth / cells.height) / aspect)));
        if (root.ff.logo.width <= 0)
            Changes.set({ store: "terminal", key: "ff.logo.width" }, w, "Width", root.pageId);
        Changes.set({ store: "terminal", key: "ff.logo.height" }, h, "Height", root.pageId);
    }

    // each add, edit, move and remove is its own change with its own notice
    function setLines(list, notice) {
        Changes.set(root.linesBind, list, "Greeting lines", root.pageId, notice);
    }

    readonly property var moduleNames: ({
        cpu: "Processor", gpu: "Graphics", memory: "Memory", disk: "Disk", kernel: "Kernel", os: "System",
        host: "Computer", uptime: "Uptime", packages: "Packages", shell: "Shell", terminal: "Terminal",
        wm: "Window manager", de: "Desktop", battery: "Battery", localip: "Local IP", board: "Board",
        swap: "Swap", display: "Display", locale: "Language", theme: "Theme", icons: "Icons", font: "Font",
        cursor: "Pointer", terminalfont: "Terminal font", datetime: "Date and time", media: "Music"
    })

    function lineTitle(line) {
        var names = { title: "User and computer", tagline: "Tagline", header: "Section header", break: "Spacer",
                      colors: "Colour swatches", raw: "Custom line" };
        if (line.kind === "module") {
            var known = root.moduleNames[String(line.module || "").toLowerCase()];
            if (known)
                return I18n.tr(known);
            var raw = String(line.label || line.module || "Info").toLowerCase();
            return raw.charAt(0).toUpperCase() + raw.slice(1);
        }
        return I18n.tr(names[line.kind] || line.label || line.kind);
    }

    function lineDetail(line) {
        var plain = s => String(s || "").replace(/\{#[^}]*\}/g, "").replace(/\{[^}]*\}/g, "").trim();
        if (line.kind === "module")
            return line.key ? I18n.tr("Shown as %1").arg(line.key) : "";
        if (line.kind === "header" || line.kind === "tagline")
            return plain(line.text);
        return "";
    }

    Component.onCompleted: Changes.ensureStore("terminal", Qt.resolvedUrl("../components/terminal/TerminalStore.qml"))

    FontMetrics {
        id: cells
        font.family: Lithic.font.mono
        font.pixelSize: 12
    }

    Image {
        id: probe
        visible: false
        asynchronous: true
        source: root.ff && root.ff.logo.kind === "image" && root.ff.logo.source
            ? "file://" + root.expand(root.ff.logo.source) : ""
    }

    // a login shell change waits on a password; past 10 s it can be cancelled
    Timer {
        interval: 1000
        repeat: true
        running: !!root.store && root.store.shellBusy
        onRunningChanged: root.shellWait = 0
        onTriggered: root.shellWait = (Date.now() - root.store.shellBusySince) / 1000
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: root.pageId
        schema: root.schema
        stateLine: root.stateLine
        controls: root.controlMap
        bodies: root.store ? ({
            prompt: root.store.starshipLoaded ? promptBody : null,
            greeting: root.store.ffLoaded ? greetingBody : null,
            picture: root.ff && root.ff.logo.kind === "image" ? pictureBody : null,
            lines: root.store.ffLoaded ? linesBody : null
        }) : ({})
        footers: ({ shell: shellFooter, greeting: greetingFooter })
    }

    Component {
        id: styleControl
        StyleControl {}
    }

    Component {
        id: actionControl
        ActionControl {}
    }

    Component {
        id: textArtControl
        TextArtControl {}
    }

    component ErrorLine: Row {
        id: line
        property string text: ""
        property var retry: null
        visible: line.text.length > 0
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, line.width - again.width - LithicTokens.space.s3)
            text: line.text
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: again
            visible: line.retry !== null
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: line.retry()
        }
    }

    Component {
        id: shellFooter
        Column {
            spacing: LithicTokens.space.s2

            Row {
                visible: !!root.store && root.store.shellBusy && root.shellWait >= 1
                width: parent.width
                spacing: LithicTokens.space.s3

                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: parent.visible
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - cancel.width - LithicTokens.space.s3 * 2 - LithicTokens.size.plaqueIcon * 2
                    text: I18n.tr("Changing the login shell. Enter your password when asked.")
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: cancel
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.shellWait >= 10
                    kind: "quiet"
                    small: true
                    text: I18n.tr("Cancel")
                    onClicked: root.store.cancelShell()
                }
            }

            ErrorLine {
                width: parent.width
                text: root.store ? root.store.shellReadError : ""
                retry: () => root.store.shellGet.running = true
            }
            ErrorLine {
                width: parent.width
                text: root.store ? root.store.starshipReadError : ""
                retry: () => root.store.starshipGet.running = true
            }
            Label {
                visible: !root.store || !root.store.shellLoaded && root.store.shellReadError.length === 0
                width: parent.width
                text: I18n.tr("Reading your login shell…")
                typeRole: "detail"
                inkRole: "fg2"
            }
        }
    }

    Component {
        id: promptBody
        PromptPreview {
            layout: root.store ? root.store.starship.layout : "pill"
        }
    }

    Component {
        id: greetingBody
        ReadoutPreview {
            store: root.store
        }
    }

    Component {
        id: greetingFooter
        Column {
            spacing: LithicTokens.space.s2

            ErrorLine {
                width: parent.width
                text: root.actionError
                retry: root.retryAction
            }
            ErrorLine {
                width: parent.width
                text: root.store ? root.store.ffReadError : ""
                retry: () => root.store.reloadGreeting()
            }
        }
    }

    Component {
        id: pictureBody
        PictureGallery {
            host: root
        }
    }

    Component {
        id: linesBody
        Column {
            spacing: LithicTokens.space.s2

            Label {
                visible: root.locked
                width: parent.width
                text: I18n.tr("This greeting style draws its own lines in the terminal, so they cannot be changed here.")
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Slab {
                visible: !root.locked
                width: parent.width

                ListEditor {
                    width: parent.width
                    model: root.ff ? root.ff.rows : []
                    reorderable: true
                    accessibleName: I18n.tr("Greeting lines")
                    addText: I18n.tr("Add a line")
                    doneText: I18n.tr("Add")
                    emptyText: I18n.tr("The greeting shows no lines.")
                    titleOf: line => root.lineTitle(line)
                    detailOf: line => root.lineDetail(line)
                    editor: Component {
                        LineEditor {
                            overlayParent: root.hub.overlayItem
                        }
                    }
                    trailing: Component {
                        Toggle {
                            property var modelData
                            property int index
                            checked: !!modelData && modelData.enabled === true
                            Accessible.name: I18n.tr("Show %1").arg(root.lineTitle(modelData || {}))
                            onToggled: {
                                var list = root.ff.rows.slice();
                                list[index] = Object.assign({}, list[index], { enabled: checked });
                                var name = root.lineTitle(list[index]);
                                root.setLines(list, checked ? I18n.tr("%1 shown").arg(name) : I18n.tr("%1 hidden").arg(name));
                            }
                        }
                    }
                    onAdded: line => root.setLines(root.ff.rows.concat([line]), I18n.tr("%1 added").arg(root.lineTitle(line)))
                    onEdited: (index, line) => {
                        var list = root.ff.rows.slice();
                        list[index] = line;
                        root.setLines(list, I18n.tr("%1 changed").arg(root.lineTitle(line)));
                    }
                    onRemoved: index => {
                        var list = root.ff.rows.slice();
                        var gone = list.splice(index, 1)[0];
                        root.setLines(list, I18n.tr("%1 removed").arg(root.lineTitle(gone)));
                    }
                    onMoved: (from, to) => {
                        var list = root.ff.rows.slice();
                        var moved = list.splice(from, 1)[0];
                        list.splice(to, 0, moved);
                        root.setLines(list, I18n.tr("%1 moved").arg(root.lineTitle(moved)));
                    }
                }
            }
        }
    }
}
