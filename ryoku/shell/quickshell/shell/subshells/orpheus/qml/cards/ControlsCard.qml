pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Bluetooth
import shell.services
import "../kit" as K
import "controls" as Pages

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})
    property string currentPage: ""
    property bool mounted: false
    property bool quietSwap: false

    readonly property bool detailPage: currentPage !== "home"
    readonly property string pageTitle: {
        if (currentPage === "wifi") return "Wi-Fi";
        if (currentPage === "bluetooth") return "Bluetooth";
        if (currentPage === "sound") return "Sound";
        if (currentPage === "display") return "Display";
        return "";
    }
    readonly property real bodyLimit: Math.max(0,
        Number(scene && scene.cardMaxHeight || 520) - 40 - (detailPage ? 52 : 0))

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight

    function validPage(value) {
        return ["home", "wifi", "bluetooth", "sound", "display"].indexOf(value) >= 0
            ? value : "home";
    }

    function pageComponent(value) {
        if (value === "wifi") return wifiComponent;
        if (value === "bluetooth") return bluetoothComponent;
        if (value === "sound") return soundComponent;
        if (value === "display") return displayComponent;
        return homeComponent;
    }

    function go(value, animate) {
        const next = validPage(value);
        if (mounted && next === currentPage)
            return;
        const motion = animate !== false && mounted && !K.Style.still;
        quietSwap = true;
        pageLoader.x = motion ? (next === "home" ? -K.Style.s6 : K.Style.s6) : 0;
        pageLoader.opacity = motion ? 0 : 1;
        currentPage = next;
        contentScroller.contentY = 0;
        pageLoader.sourceComponent = pageComponent(next);
        Qt.callLater(function() {
            root.quietSwap = false;
            pageLoader.x = 0;
            pageLoader.opacity = 1;
        });
    }

    function back() {
        go("home", true);
    }

    onArgsChanged: if (mounted) go(args && args.page ? args.page : "home", true)
    Component.onCompleted: {
        mounted = true;
        go(args && args.page ? args.page : "home", false);
    }

    Item {
        id: radioSlot

        readonly property bool shown: root.currentPage === "wifi" || root.currentPage === "bluetooth"
        visible: shown
        implicitWidth: radioRow.implicitWidth
        implicitHeight: radioRow.implicitHeight
        width: shown ? implicitWidth : 0
        height: shown ? implicitHeight : 0

        Row {
            id: radioRow
            spacing: K.Style.s2

            K.IconButton {
                visible: root.currentPage === "wifi"
                icon: "refresh"
                enabled: Network.wifiRadio
                onClicked: Network.refresh()
            }

            K.Switch {
                checked: root.currentPage === "wifi"
                    ? Network.wifiRadio
                    : BluetoothPower.enabled
                enabled: root.currentPage === "wifi"
                    ? Network.wifiPresent : BluetoothPower.available
                onToggled: value => {
                    if (root.currentPage === "wifi")
                        Network.setWifiEnabled(value);
                    else
                        BluetoothPower.setEnabled(value);
                }
            }
        }
    }

    Component {
        id: homeComponent
        Pages.HomePage {
            scene: root.scene
            card: root
            shown: root.shown && root.currentPage === "home"
        }
    }

    Component {
        id: wifiComponent
        Pages.WifiPage {
            scene: root.scene
            card: root
            shown: root.shown && root.currentPage === "wifi"
        }
    }

    Component {
        id: bluetoothComponent
        Pages.BluetoothPage {
            scene: root.scene
            card: root
            shown: root.shown && root.currentPage === "bluetooth"
        }
    }

    Component {
        id: soundComponent
        Pages.SoundPage {
            scene: root.scene
            card: root
            shown: root.shown && root.currentPage === "sound"
        }
    }

    Component {
        id: displayComponent
        Pages.DisplayPage {
            scene: root.scene
            card: root
            shown: root.shown && root.currentPage === "display"
        }
    }

    K.Sheet {
        id: sheet

        pageWidth: 360
        title: root.pageTitle
        backVisible: root.detailPage
        trailing: root.currentPage === "wifi" || root.currentPage === "bluetooth"
            ? [radioSlot] : []
        onBack: root.back()

        Item {
            id: bodyViewport

            width: parent.width
            implicitHeight: Math.min(root.bodyLimit,
                pageLoader.item ? pageLoader.item.implicitHeight : 0)
            height: implicitHeight
            clip: true

            K.Scroller {
                id: contentScroller
                anchors.fill: parent

                Loader {
                    id: pageLoader

                    width: contentScroller.width
                    height: item ? item.implicitHeight : 0

                    Behavior on x {
                        enabled: !root.quietSwap && !K.Style.still
                        NumberAnimation {
                            duration: K.Style.effects
                            easing.type: Easing.BezierSpline
                            easing.bezierCurve: K.Style.effectsCurve
                        }
                    }
                    Behavior on opacity {
                        enabled: !root.quietSwap && !K.Style.still
                        NumberAnimation {
                            duration: K.Style.effects
                            easing.type: Easing.BezierSpline
                            easing.bezierCurve: K.Style.effectsCurve
                        }
                    }
                }
            }
        }
    }
}
