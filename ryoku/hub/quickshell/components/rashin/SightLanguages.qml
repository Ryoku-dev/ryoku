import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../installed-apps"
import "sight.js" as Sight

Column {
    id: root
    readonly property bool wide: true

    required property var store
    property var row: null
    property var value: []
    property var commit: null
    property int selected: 0
    readonly property string code: Sight.languages[root.selected].code
    readonly property bool chosen: Array.from(root.value || []).indexOf(root.code) >= 0
    readonly property bool installed: root.store.installed.indexOf(root.code) >= 0
    width: LithicTokens.size.settingsControlMax
    spacing: LithicTokens.space.s2

    Row {
        width: Math.min(parent.width, LithicTokens.size.settingsControlMax + languageToggle.implicitWidth + spacing)
        spacing: LithicTokens.space.s3
        Picker {
            id: picker
            width: parent.width - languageToggle.width - parent.spacing
            options: Sight.languages.map(l => I18n.tr(l.label))
            currentIndex: root.selected
            Accessible.name: I18n.tr("Text recognition language")
            onActivated: index => root.selected = index
        }
        Toggle {
            id: languageToggle
            anchors.verticalCenter: parent.verticalCenter
            checked: root.chosen
            enabled: root.store.languagesReady && !root.store.job.running
            Accessible.name: I18n.tr("Use %1 for text recognition").arg(I18n.tr(Sight.languages[root.selected].label))
            onToggled: {
                var want = checked;
                checked = Qt.binding(() => root.chosen);
                var next = Array.from(root.value || []).filter(code => code !== root.code);
                if (want)
                    next.push(root.code);
                if (root.commit)
                    root.commit(next);
            }
        }
    }
    Label {
        width: parent.width
        text: !root.store.languagesReady ? I18n.tr("Reading installed languages")
            : root.installed ? I18n.tr("Installed") : I18n.tr("Installs when enabled; your password is required")
        typeRole: "detail"
        inkRole: "fg3"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    JobLine {
        job: root.store.job
        target: "sight-languages"
        doing: I18n.tr("Installing text recognition languages")
    }
    Label {
        visible: root.store.languageError !== ""
        width: parent.width
        text: root.store.languageError
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Plaque {
        visible: root.store.languageError !== "" && !root.store.job.running
        text: I18n.tr("Refresh languages")
        small: true
        kind: "quiet"
        glyph: "refresh"
        onClicked: root.store.refresh()
    }
}
