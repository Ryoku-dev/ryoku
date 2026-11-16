import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

// Whether this machine keeps its signed card online for friends' binders. The card studio's
// Share tab and its Badge tab show the same switch.
Column {
    id: root
    property var hub: null
    // the Badge tab already prints the badge tool's errors
    property bool showsError: true
    // what the reader asked for, shown while the badge tool applies it
    property bool wanted: false
    spacing: LithicTokens.space.s2

    function set(on) {
        root.wanted = on;
        Badge.setOnline(on);
        // a press flips the switch by hand and drops its binding; the badge status owns it again
        toggle.checked = Qt.binding(() => Badge.job === "online" ? root.wanted : Badge.online);
    }

    SettingRow {
        id: row
        width: parent.width
        first: true
        title: I18n.tr("Keep my card up to date for friends")
        activatable: Badge.job === ""
        onActivated: root.set(!Badge.online)
        IconButton {
            id: why
            anchors.verticalCenter: parent.verticalCenter
            glyph: "info"
            accessibleName: I18n.tr("What keeping your card up to date means")
            onClicked: explanation.opened ? explanation.close() : explanation.open()
        }
        Toggle {
            id: toggle
            anchors.verticalCenter: parent.verticalCenter
            checked: Badge.job === "online" ? root.wanted : Badge.online
            enabled: Badge.job === ""
            Accessible.name: row.title
            onToggled: root.set(toggle.checked)
        }
    }
    Label {
        width: parent.width
        visible: root.showsError && Badge.errorFrom === "online" && Badge.error !== ""
        text: Badge.error
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Popover {
        id: explanation
        parent: root.hub ? root.hub.overlayItem : root
        anchorItem: why
        opener: why
        menuFocus: false
        CutRect {
            implicitWidth: LithicTokens.size.menuWidthMax
            implicitHeight: words.implicitHeight + LithicTokens.space.s4 * 2
            fill: Lithic.stone4
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.menu
            Label {
                id: words
                x: LithicTokens.space.s4
                y: LithicTokens.space.s4
                width: parent.width - LithicTokens.space.s4 * 2
                text: I18n.tr("Off: your card is just the picture you send. On: your card also lives online, so friends who added it see your changes without you sending it again. Only your computer can change your card. If you don't change your card for 3 months it is taken offline, but friends keep the copy they have. Turning this off takes it offline too. Friends' binders check for changes now and then; the time of the next check is shown in the binder.")
                typeRole: "detail"
                inkRole: "fg"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }
}
