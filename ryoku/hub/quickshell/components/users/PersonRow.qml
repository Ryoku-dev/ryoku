import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// a setting row of one person or profile, with its working mark, note and error
Column {
    id: pr

    // the PersonSlab or ProfileSlab: store, page, spot
    required property var owner
    property string rowId: ""
    property alias title: sr.title
    property alias detail: sr.detail
    property alias unavailable: sr.unavailable
    property alias activatable: sr.activatable
    property var bind: null
    property bool busy: false
    property string note: ""
    property string error: pr.bind ? Changes.error(pr.bind) : ""
    default property alias control: sr.control
    readonly property alias settingRow: sr
    signal activated()

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s1

    Component.onCompleted: {
        if (pr.owner.spot && pr.owner.page && pr.rowId !== "")
            pr.owner.page.rowItems[pr.rowId] = pr;
    }
    Component.onDestruction: {
        if (pr.owner.page && pr.rowId !== "" && pr.owner.page.rowItems[pr.rowId] === pr)
            delete pr.owner.page.rowItems[pr.rowId];
    }

    SettingRow {
        id: sr
        width: parent.width
        highlighted: pr.owner.spot && !!pr.owner.page && pr.rowId !== "" && pr.owner.page.highlightId === pr.rowId
        onActivated: pr.activated()
    }

    Row {
        visible: (pr.busy && pr.owner.store.slow) || pr.note.length > 0
        spacing: LithicTokens.space.s2

        Working {
            visible: pr.busy
            anchors.verticalCenter: parent.verticalCenter
            active: visible
            size: LithicTokens.size.plaqueIconSmall
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: pr.busy ? I18n.tr("Applying. This may ask for an administrator's password.") : pr.note
            typeRole: "detail"
            inkRole: "fg2"
        }
        Plaque {
            visible: pr.busy && pr.owner.store.lengthy
            // hidden plaques keep keyboard focus and would act on Return, so they are disabled with their row
            enabled: pr.busy && pr.owner.store.lengthy
            kind: "quiet"
            small: true
            text: I18n.tr("Cancel")
            onClicked: pr.owner.store.cancel()
        }
    }

    Row {
        visible: pr.error.length > 0
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - (retry.visible ? retry.width + LithicTokens.space.s3 : 0))
            text: pr.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            visible: !!pr.bind
            enabled: pr.error.length > 0
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: Changes.retry(pr.bind)
        }
    }
}
