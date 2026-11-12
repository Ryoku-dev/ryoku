import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// one greeting line: a new one picks what it shows; an existing one renames
// its label or text where it has one
Column {
    id: editor

    property var item: ({})
    property Item overlayParent: null
    readonly property bool adding: editor.item.kind === undefined
    readonly property var kinds: [
        { key: "__header", label: "Section header" },
        { key: "__break", label: "Spacer" },
        { key: "__colors", label: "Colour swatches" },
        { key: "cpu", label: "CPU" }, { key: "gpu", label: "GPU" },
        { key: "memory", label: "Memory" }, { key: "disk", label: "Disk" },
        { key: "kernel", label: "Kernel" }, { key: "os", label: "OS" },
        { key: "host", label: "Host" }, { key: "uptime", label: "Uptime" },
        { key: "packages", label: "Packages" }, { key: "shell", label: "Shell" },
        { key: "terminal", label: "Terminal" }, { key: "wm", label: "Window manager" },
        { key: "de", label: "Desktop" }, { key: "battery", label: "Battery" },
        { key: "localip", label: "Local IP" }, { key: "board", label: "Board" }
    ]
    property int kindIndex: 3
    readonly property var kind: editor.kinds[editor.kindIndex]
    readonly property string shownKind: editor.adding ? (editor.kind.key === "__header" ? "header"
        : editor.kind.key === "__break" ? "break" : editor.kind.key === "__colors" ? "colors" : "module")
        : String(editor.item.kind || "")
    readonly property bool hasText: ["module", "header", "tagline"].indexOf(editor.shownKind) >= 0
    readonly property bool valid: !editor.hasText || editor.shownKind === "tagline" || textField.text.trim().length > 0

    function defaultText() {
        if (editor.shownKind === "header")
            return I18n.tr("SECTION");
        return I18n.tr(editor.kind.label).toUpperCase();
    }

    function value() {
        var t = textField.text.trim();
        if (!editor.adding) {
            var out = Object.assign({}, editor.item);
            if (out.kind === "module")
                out.key = t;
            else if (out.kind === "header" || out.kind === "tagline") {
                if (out.kind === "tagline" && t !== String(out.text || ""))
                    out.textEdited = true;
                out.text = t;
            }
            return out;
        }
        var k = editor.kind.key;
        if (k === "__header")
            return { kind: "header", enabled: true, text: t, label: "Section header" };
        if (k === "__break")
            return { kind: "break", enabled: true, label: "Spacer" };
        if (k === "__colors")
            return { kind: "colors", enabled: true, label: "Colour swatches", raw: { type: "colors", symbol: "circle" } };
        return { kind: "module", enabled: true, module: k, key: t, label: editor.kind.label.toUpperCase() };
    }

    spacing: LithicTokens.space.s3

    Picker {
        visible: editor.adding
        width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
        label: I18n.tr("What the line shows")
        options: editor.kinds.map(k => I18n.tr(k.label))
        currentIndex: editor.kindIndex
        overlayParent: editor.overlayParent
        onActivated: index => {
            editor.kindIndex = index;
            textField.text = editor.defaultText();
        }
    }

    TextField {
        id: textField
        visible: editor.hasText
        width: parent.width
        label: editor.shownKind === "module" ? I18n.tr("Label") : I18n.tr("Text")
        helpText: editor.shownKind === "module" ? I18n.tr("The word in the accent colour before the value")
            : ""
        mono: true
        text: editor.adding ? editor.defaultText()
            : editor.item.kind === "module" ? String(editor.item.key || "") : String(editor.item.text || "")
    }

    Label {
        visible: !editor.hasText
        width: parent.width
        text: editor.adding ? I18n.tr("This line has nothing to name.")
            : I18n.tr("This line has nothing to rename. Turn it off or remove it instead.")
        typeRole: "body"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
