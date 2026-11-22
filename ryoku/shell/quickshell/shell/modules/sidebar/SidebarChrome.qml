pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    required property var screen
    required property real s
    required property bool active
    signal closeRequested()

    readonly property string requestedPage: SidebarState.activeTab(root.screen)
    readonly property bool pluginsEnabled: SidebarState.elementVisible("pluginCards")
    readonly property var pluginCards: pluginHost.cards()
    readonly property string page: root.requestedPage === "plugins"
        && (!root.pluginsEnabled || root.pluginCards.length === 0) ? "controls" : root.requestedPage
    readonly property bool detail: root.page !== "controls"
    readonly property bool identity: SidebarState.elementVisible("identity")
    readonly property real pad: Math.round(QsLook.pad * root.s)
    readonly property real headGap: Math.round((QsLook.lithic ? 10 : 12) * root.s)
    readonly property real fittedHeight: root.pad * 2 + head.height + root.headGap + board.implicitHeight
    onPluginsEnabledChanged: if (!root.pluginsEnabled && root.requestedPage === "plugins") root.home()

    function home(): void {
        SidebarState.selectTab(root.screen, "controls");
    }
    function back(): bool {
        return board.back();
    }

    SidebarPlugins {
        id: pluginHost
        active: root.active && root.pluginsEnabled
    }

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
        enabled: root.active
    }
    FileView {
        id: hostFile
        path: "/etc/hostname"
        printErrors: false
        onLoaded: root.hostName = String(hostFile.text() || "").trim()
    }
    FileView {
        id: uptimeFile
        path: "/proc/uptime"
        printErrors: false
        onLoaded: {
            const up = Number(String(uptimeFile.text()).split(" ")[0]);
            if (isFinite(up))
                root.bootMs = Date.now() - up * 1000;
        }
    }
    property real bootMs: 0
    property string hostName: ""
    onActiveChanged: if (root.active) uptimeFile.reload()

    readonly property string userName: Quickshell.env("USER") || ""
    readonly property string upFor: {
        if (root.bootMs <= 0)
            return "";
        const minutes = Math.max(0, Math.floor((clock.date.getTime() - root.bootMs) / 60000));
        if (minutes < 60)
            return I18n.tr("up %1 min").arg(minutes);
        const hours = Math.floor(minutes / 60);
        if (hours < 24)
            return I18n.tr("up %1 h %2 min").arg(hours).arg(minutes % 60);
        return I18n.tr("up %1 d %2 h").arg(Math.floor(hours / 24)).arg(hours % 24);
    }
    readonly property string proof: [root.hostName, root.upFor].filter(part => part.length > 0).join(" · ")
    readonly property string pageTitle: root.page === "wifi" ? I18n.tr("Wi-Fi")
        : root.page === "bluetooth" ? I18n.tr("Bluetooth")
        : root.page === "plugins" ? I18n.tr("Extension cards")
        : I18n.tr("Quick settings")
    readonly property string pageGloss: root.page === "wifi" ? "無線"
        : root.page === "bluetooth" ? "近接" : root.page === "plugins" ? "拡張" : "早設"

    Item {
        id: head
        x: root.pad
        y: root.pad
        width: parent.width - root.pad * 2
        height: (QsLook.lithic ? masthead.height + ruler.height + Math.round(10 * root.s) : 0) + row.height

        Item {
            id: masthead
            visible: QsLook.lithic
            width: parent.width
            height: QsLook.lithic ? Math.round(14 * root.s) : 0

            Row {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(6 * root.s)

                Text {
                    text: "§"
                    color: Lithic.sel
                    font.family: QsLook.mono
                    font.pixelSize: Math.round(10 * root.s)
                    renderType: Text.NativeRendering
                }
                Text {
                    text: root.detail ? I18n.tr("Quick settings") + " / " + root.pageTitle : I18n.tr("Quick settings")
                    color: Lithic.fg2
                    font.family: QsLook.mono
                    font.pixelSize: Math.round(9 * root.s)
                    font.capitalization: Font.AllUppercase
                    font.letterSpacing: Look.tracking * root.s
                    renderType: Text.NativeRendering
                }
                Text {
                    visible: UiPreferences.showJapaneseGloss
                    text: root.pageGloss
                    color: Lithic.fg3
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(10 * root.s)
                    renderType: Text.NativeRendering
                }
            }
            Text {
                visible: root.identity
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: Qt.formatDateTime(clock.date, I18n.tr("ddd d MMM")) + " · " + TimeFormat.format(clock.date)
                color: Lithic.fg2
                font.family: QsLook.mono
                font.pixelSize: Math.round(9 * root.s)
                font.capitalization: Font.AllUppercase
                font.letterSpacing: Look.tracking * root.s
                renderType: Text.NativeRendering
            }
        }
        Item {
            id: ruler
            visible: QsLook.lithic
            anchors.top: masthead.bottom
            anchors.topMargin: Math.round(4 * root.s)
            width: parent.width
            height: QsLook.lithic ? Math.round(7 * root.s) : 0
            readonly property real step: Math.round(8 * root.s)

            Rectangle {
                anchors.bottom: parent.bottom
                width: parent.width
                height: Math.max(1, Math.round(root.s))
                color: Lithic.vein
            }
            Repeater {
                model: ruler.visible ? Math.max(0, Math.floor(ruler.width / ruler.step) + 1) : 0
                delegate: Rectangle {
                    required property int index
                    x: Math.round(index * ruler.step)
                    anchors.bottom: parent.bottom
                    width: Math.max(1, Math.round(root.s))
                    height: Math.round((index % 5 === 0 ? 7 : 4) * root.s)
                    color: index % 5 === 0 ? Lithic.fg3 : Lithic.alpha(Lithic.fg3, 0.55)
                }
            }
        }

        Item {
            id: row
            anchors.bottom: parent.bottom
            width: parent.width
            height: Math.round((QsLook.material && !root.detail && root.identity ? 58 : QsLook.lithic ? 36 : 40) * root.s)

            CornerButton {
                id: backButton
                visible: root.detail
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                glyph: "arrow_back"
                subtle: true
                Accessible.name: I18n.tr("Back to Quick settings")
                onClicked: root.home()
            }

            // Material leads with the time the way Android's shade does
            Column {
                id: clockFace
                visible: QsLook.material && !root.detail && root.identity
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(1 * root.s)

                Text {
                    text: TimeFormat.format(clock.date)
                    color: Lithic.fg
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(30 * root.s)
                    font.weight: Font.Normal
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                }
                Text {
                    text: [Qt.formatDateTime(clock.date, I18n.tr("ddd, d MMM")), root.upFor].filter(part => part.length > 0).join(" · ")
                    color: Lithic.fg2
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(12 * root.s)
                    renderType: Text.NativeRendering
                }
            }

            Avatar {
                id: avatar
                visible: !root.detail && root.identity && !QsLook.material
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                size: QsLook.lithic ? 32 : 34
                userName: root.userName
            }

            Column {
                visible: !clockFace.visible
                anchors.left: root.detail ? backButton.right : avatar.visible ? avatar.right : parent.left
                anchors.leftMargin: root.detail || avatar.visible ? Math.round(10 * root.s) : 0
                anchors.right: tail.left
                anchors.rightMargin: Math.round(10 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(2 * root.s)

                Text {
                    width: parent.width
                    text: root.detail || !root.identity ? root.pageTitle : root.userName
                    color: Lithic.fg
                    font.family: QsLook.sans
                    font.pixelSize: Math.round((root.detail ? 16 : 14) * root.s)
                    font.weight: QsLook.adwaita || QsLook.fluent ? Font.DemiBold : Font.Medium
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
                Text {
                    width: parent.width
                    visible: !root.detail && root.identity && root.proof.length > 0
                    text: root.proof
                    color: Lithic.fg2
                    font.family: QsLook.lithic ? QsLook.mono : QsLook.sans
                    font.pixelSize: Math.round((QsLook.lithic ? 10 : 12) * root.s)
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
            }

            Row {
                id: tail
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(8 * root.s)

                Text {
                    visible: QsLook.adwaita && !root.detail && root.identity
                    anchors.verticalCenter: parent.verticalCenter
                    text: TimeFormat.format(clock.date)
                    color: Lithic.fg2
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(13 * root.s)
                    font.weight: Font.Medium
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                }
                CornerButton {
                    visible: !root.detail && root.pluginsEnabled && root.pluginCards.length > 0
                    anchors.verticalCenter: parent.verticalCenter
                    s: root.s
                    glyph: "extension"
                    subtle: true
                    Accessible.name: I18n.tr("Extension cards")
                    onClicked: SidebarState.selectTab(root.screen, "plugins")
                }
                Avatar {
                    visible: QsLook.material && !root.detail && root.identity
                    anchors.verticalCenter: parent.verticalCenter
                    s: root.s
                    size: 40
                    userName: root.userName

                    HoverHandler {
                        id: avatarHover
                    }
                    CornerTip {
                        s: root.s
                        visible: avatarHover.hovered && root.proof.length > 0
                        text: root.userName + " · " + root.proof
                    }
                }
            }
        }
    }

    ControlsBoard {
        id: board
        x: root.pad
        y: head.y + head.height + root.headGap
        width: parent.width - root.pad * 2
        height: Math.max(0, parent.height - y - root.pad)
        s: root.s
        screen: root.screen
        card: root
        active: root.active
        page: root.page
        heroIdentity: false
        pluginCards: root.pluginCards
        bleed: root.pad
        onRequestClose: root.closeRequested()
    }
}
