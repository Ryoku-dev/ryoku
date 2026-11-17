import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "apps.js" as Apps

// one installed app: where it came from and its size, whether it shows in app search, Remove
Column {
    id: item

    required property var app
    required property int index
    // the page: store, hidden, canHide, planningId, blocked, setShown(app, shown), askRemove(app), detailOf(app, shown)
    required property var host

    readonly property bool shown: item.host.hidden.indexOf(item.app.id) < 0
    readonly property var how: Apps.removeCommand(item.app)
    readonly property bool busy: (item.host.store.job.running && item.host.store.job.target === item.app.id)
        || item.host.planningId === item.app.id
    readonly property var needs: item.host.blocked && item.host.blocked.id === item.app.id ? item.host.blocked.names : []

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s1

    SettingRow {
        width: parent.width
        first: item.index === 0
        title: item.app.name
        detail: item.host.detailOf(item.app, item.shown)

        Plaque {
            visible: item.how !== null
            kind: "quiet"
            small: true
            glyph: item.how && item.how.kind === "terminal" ? "terminal" : "delete"
            text: I18n.tr("Remove")
            busy: item.busy
            enabled: !item.host.store.job.running && item.host.planningId === ""
            accessibleName: I18n.tr("Remove %1").arg(item.app.name)
            onClicked: item.host.askRemove(item.app)
        }
        Toggle {
            checked: item.shown
            enabled: item.host.canHide
            Accessible.name: I18n.tr("Show %1 in app search").arg(item.app.name)
            onToggled: item.host.setShown(item.app, checked)
        }
    }

    JobLine {
        job: item.host.store.job
        target: item.app.id
        doing: I18n.tr("Removing %1").arg(item.app.name)
    }

    // other packages need it, so the package manager would refuse; said where Remove was pressed
    Label {
        width: parent.width
        visible: opacity > 0
        opacity: item.needs.length > 0 ? 1 : 0
        text: item.needs.length === 1
            ? I18n.tr("%1 needs it, so it cannot be removed on its own. Remove that first.").arg(item.needs[0])
            : item.needs.length > 1
            ? I18n.tr("%1 need it, so it cannot be removed on its own. Remove those first.").arg(item.host.joined(item.needs))
            : ""
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
        Behavior on opacity {
            NumberAnimation { duration: Tone.quick; easing.type: Easing.OutCubic }
        }
    }
}
