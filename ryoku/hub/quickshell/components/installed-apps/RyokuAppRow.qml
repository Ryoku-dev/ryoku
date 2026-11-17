import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// one of Ryoku's own apps, pinned above the rest: its version and size, an update when one is
// ready, Remove (never for one built into the desktop), or where to get it when it is not here
Column {
    id: item

    required property var app
    required property int index
    // the page: store, hidden, canHide, setShown(app, shown), askRyoku(app), openStore(route), ryokuDetail(app, shown)
    required property var host

    readonly property string target: "ryoku:" + item.app.id
    readonly property bool busy: item.host.store.job.running && item.host.store.job.target === item.target
    readonly property bool shown: item.host.hidden.indexOf(item.app.desktopId) < 0
    // ryoku-host updates and removes through gpk's progress events, which an older gpk lacks
    readonly property bool gpkOld: item.host.store.gpkOld
    readonly property bool writes: item.app.installed && (item.app.update || item.app.removable)

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s1

    SettingRow {
        width: parent.width
        first: item.index === 0
        title: item.app.name
        detail: item.host.ryokuDetail(item.app, item.shown)

        Plaque {
            visible: item.app.installed && item.app.update
            kind: "tonal"
            small: true
            glyph: "download"
            text: I18n.tr("Update")
            busy: item.busy && item.host.store.job.action === "update"
            enabled: !item.host.store.job.running && !item.gpkOld
            accessibleName: I18n.tr("Update %1 to %2").arg(item.app.name).arg(item.app.latest)
            onClicked: item.host.store.changeRyoku(item.app.id, "update")
        }
        Plaque {
            visible: item.app.installed && item.app.removable
            kind: "quiet"
            small: true
            glyph: "delete"
            text: I18n.tr("Remove")
            busy: item.busy && item.host.store.job.action === "remove"
            enabled: !item.host.store.job.running && !item.gpkOld
            accessibleName: I18n.tr("Remove %1").arg(item.app.name)
            onClicked: item.host.askRyoku(item.app)
        }
        Plaque {
            visible: item.app.installed && item.app.builtin
            kind: "quiet"
            small: true
            glyph: "dns"
            text: I18n.tr("Sources")
            accessibleName: I18n.tr("Choose where %1 gets its items").arg(item.app.name)
            onClicked: item.host.openStore("sources")
        }
        Plaque {
            visible: item.app.installed && item.app.builtin
            kind: "quiet"
            small: true
            glyph: "open_in_new"
            text: I18n.tr("Open")
            accessibleName: I18n.tr("Open %1").arg(item.app.name)
            onClicked: item.host.openStore("apps")
        }
        Plaque {
            visible: !item.app.installed
            kind: "quiet"
            small: true
            glyph: "open_in_new"
            text: I18n.tr("Get")
            accessibleName: I18n.tr("Get %1 in RyoStore").arg(item.app.name)
            onClicked: item.host.openStore("ryoku-apps")
        }
        Toggle {
            visible: item.app.installed && item.app.desktopId !== ""
            checked: item.shown
            enabled: item.host.canHide
            Accessible.name: I18n.tr("Show %1 in app search").arg(item.app.name)
            onToggled: item.host.setShown({ id: item.app.desktopId, name: item.app.name }, checked)
        }
    }

    JobLine {
        job: item.host.store.job
        target: item.target
        doing: item.host.store.job.action === "update"
            ? I18n.tr("Updating %1").arg(item.app.name)
            : I18n.tr("Removing %1").arg(item.app.name)
    }

    Label {
        width: parent.width
        visible: item.gpkOld && item.writes
        text: I18n.tr("Needs gpk 0.7.0, which comes with the next Ryoku update.")
        typeRole: "detail"
        inkRole: "fg3"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
