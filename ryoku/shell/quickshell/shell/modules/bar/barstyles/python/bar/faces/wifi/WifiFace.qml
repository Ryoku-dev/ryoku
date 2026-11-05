import QtQuick
import QtQuick.Window
import Quickshell
import shell.services
import "../../../reusables"
import "../../../"

Item {
    id: root

    property var module: null
    property var widget: module

    property string orientation: (activeTarget && activeTarget.orientation !== undefined) ? activeTarget.orientation : "horizontal"
    readonly property bool isVertical: orientation === "vertical"

    readonly property var activeTarget: widget || module
    readonly property bool isCompact: activeTarget ? activeTarget.isCompact : false
    readonly property var barWindow: activeTarget ? activeTarget.barWindow : null
    readonly property bool isPreview: activeTarget ? Boolean(activeTarget.isPreview) : false

    function s(val) {
        if (barWindow && typeof barWindow.s === "function") return barWindow.cs(val);
        if (activeTarget && typeof activeTarget.s === "function") return activeTarget.s(val);
        if (typeof Scaler !== "undefined" && typeof Scaler.s === "function") return Math.round(Scaler.s(val));
        return val;
    }

    property int configRevision: 0

    Connections {
        target: (typeof Config !== "undefined") ? Config : null
        function onSettingsLoaded() { root.configRevision++; }
        function onRawSettingsChanged() { root.configRevision++; }
    }

    property string wifiStyle: {
        if (module && module.variant) return module.variant;
        if (widget && widget !== root && widget.wifiStyle !== undefined) return widget.wifiStyle;
        if (module && module.wifiStyle !== undefined) return module.wifiStyle;
        let dummy = configRevision;
        if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar) {
            let bs = Config.rawSettings.bar;
            if (bs.wifiStyle) return bs.wifiStyle;
            if (bs.wifi && bs.wifi.style) return bs.wifi.style;
        }
        return "button";
    }

    property bool showIcon: {
        if (widget && widget !== root && widget.wifiShowIcon !== undefined) return widget.wifiShowIcon;
        if (widget && widget !== root && widget.showIcon !== undefined) return widget.showIcon;
        if (module && module.wifiShowIcon !== undefined) return module.wifiShowIcon;
        if (module && module.showIcon !== undefined) return module.showIcon;
        let dummy = configRevision;
        if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar) {
            let bs = Config.rawSettings.bar;
            if (bs.wifiShowIcon !== undefined) return Boolean(bs.wifiShowIcon);
            if (bs.wifi && bs.wifi.showIcon !== undefined) return Boolean(bs.wifi.showIcon);
        }
        return true;
    }

    property bool showName: {
        if (widget && widget !== root && widget.wifiShowName !== undefined) return widget.wifiShowName;
        if (widget && widget !== root && widget.showName !== undefined) return widget.showName;
        if (module && module.wifiShowName !== undefined) return module.wifiShowName;
        if (module && module.showName !== undefined) return module.showName;
        let dummy = configRevision;
        if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar) {
            let bs = Config.rawSettings.bar;
            if (bs.wifiShowName !== undefined) return Boolean(bs.wifiShowName);
            if (bs.wifi && bs.wifi.showName !== undefined) return Boolean(bs.wifi.showName);
        }
        return true;
    }

    property string ethStatus: isPreview || Network.kind !== "ethernet" ? "Ethernet" : "Connected"
    property string wifiStatus: isPreview || Network.wifiRadio ? "Enabled" : "Off"
    property string wifiSsid: isPreview ? "Home-WiFi" : Network.activeSsid
    property bool isWifiOn: isPreview || Network.wifiRadio
    property bool showEthernet: !isPreview && Network.kind === "ethernet"
    property bool showLayout: (!barWindow || isPreview) ? true : ((!module || module.moduleActive) && barWindow.isStartupReady && barWindow.isDataReady)
    readonly property var wifiPill: pillLoader.item

    property string wifiIcon: {
        if (isPreview)
            return "󰤨";
        if (!Network.wifiRadio)
            return "󰤮";
        if (Network.kind !== "wifi")
            return "󰤯";
        const signal = Math.round(Network.level * 100);
        if (signal >= 80) return "󰤨";
        if (signal >= 60) return "󰤥";
        if (signal >= 40) return "󰤢";
        if (signal >= 20) return "󰤟";
        return "󰤯";
    }


    property string effectiveIcon: root.showEthernet ? "󰈀" : root.wifiIcon
    property string effectiveName: root.showEthernet ? root.ethStatus : ((root.isWifiOn ? (root.wifiSsid !== "" ? root.wifiSsid : "On") : "Off"))
    readonly property bool isActive: root.showEthernet ? (root.ethStatus === "Connected") : root.isWifiOn

    readonly property real pillLength: pillLoader.item ? (isVertical ? pillLoader.item.height : pillLoader.item.width) : 0
    readonly property real textLength: isVertical ? textLayout.implicitHeight : textLayout.implicitWidth

    property real targetLength: {
        if (module && !module.moduleActive) return 0;
        if (root.wifiStyle === "text") {
            return (textLength > 0) ? (textLength + s(root.isCompact ? (isVertical ? 14 : 12) : 16)) : 0;
        }
        return (pillLength > 0) ? (pillLength + s(root.isCompact ? 8 : 10)) : 0;
    }
    property bool isFaceVisible: showLayout && targetLength > 0

    function syncImplicitSize() {
        let cross = module ? module.thickness : (parent ? (isVertical ? parent.width : parent.height) : 0);
        implicitWidth = isVertical ? cross : targetLength;
        implicitHeight = isVertical ? targetLength : cross;
    }

    onTargetLengthChanged: syncImplicitSize()
    onIsVerticalChanged: syncImplicitSize()
    onModuleChanged: syncImplicitSize()
    onParentChanged: syncImplicitSize()

    Timer {
        running: true
        interval: 0
        onTriggered: root.syncImplicitSize()
    }

    transform: Translate {
        x: (root.isVertical || root.showLayout) ? 0 : s(60)
        Behavior on x { NumberAnimation { duration: 800; easing.type: Easing.OutQuint } }
    }

    MouseArea {
        id: textMouseArea
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        enabled: root.wifiStyle === "text" && !root.isPreview
        onClicked: Quickshell.execDetached(["bash", "-c", Caching.payloadDir + "/scripts/qs_manager.sh toggle network wifi"])
    }

    AxisFlow {
        id: textLayout
        vertical: root.isVertical
        horizontalItemAlignment: Grid.AlignHCenter
        verticalItemAlignment: Grid.AlignVCenter
        visible: root.wifiStyle === "text"
        anchors.centerIn: parent
        spacing: root.isVertical ? s(root.isCompact ? 2 : 3) : s(root.isCompact ? 5 : 6)
        opacity: root.showLayout ? 1.0 : 0.0
        Behavior on opacity { NumberAnimation { duration: 400; easing.type: Easing.OutCubic } }

        Item {
            visible: root.showIcon
            width: root.isVertical ? textIcon.implicitWidth : s(root.isCompact ? 18 : 20)
            height: root.isVertical ? textIcon.implicitHeight : width

            Text {
                id: textIcon
                anchors.centerIn: parent
                text: root.effectiveIcon
                font.family: ThemeBackend.iconFont
                font.pixelSize: s(root.isCompact ? 14 : 15)
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
                color: textMouseArea.containsMouse ? Qt.lighter(ThemeBackend.blue, 1.15) : (root.isActive ? ThemeBackend.blue : ThemeBackend.subtext0)
                Behavior on color { ColorAnimation { duration: 150 } }
            }
        }

        Text {
            visible: root.showName
            text: root.isVertical ? (root.effectiveName.length > 6 ? (root.effectiveName.substring(0, 5) + "…") : root.effectiveName) : root.effectiveName
            font.family: ThemeBackend.fontFamily
            font.pixelSize: root.isVertical ? s(root.isCompact ? 9 : 10) : s(root.isCompact ? 11 : 12)
            font.bold: true
            color: textMouseArea.containsMouse ? Qt.lighter(ThemeBackend.text, 1.15) : ThemeBackend.text
            Behavior on color { ColorAnimation { duration: 150 } }
        }
    }

    Loader {
        id: pillLoader
        visible: root.wifiStyle !== "text"
        anchors.centerIn: parent
        sourceComponent: root.isVertical ? sideButtonComponent : topButtonComponent
    }

    Component {
        id: topButtonComponent

        ClickButton {
            id: wifiPill
            property bool initAnimTrigger: root.showLayout
            property bool isActive: root.isActive

            height: s(root.isCompact ? 28 : 30)
            maxWidth: s(root.isCompact ? 156 : 160)
            cornerRadius: Math.max(0, ThemeBackend.borderRadius - s(2))
            horizontalPadding: s(root.isCompact ? 10 : 12)
            buttonIcon: root.showIcon ? root.effectiveIcon : ""
            iconFontSize: s(root.isCompact ? 14 : 15)
            buttonText: root.showName ? root.effectiveName : ""
            textFontSize: s(root.isCompact ? 11 : 12)
            accentColor: isActive ? (root.isCompact ? Qt.lighter(ThemeBackend.blue, 1.08) : ThemeBackend.blue) : (root.isCompact ? Qt.lighter(ThemeBackend.surface0, 1.18) : ThemeBackend.surface0)
            textColor: isActive ? ThemeBackend.base : (root.isCompact ? Qt.lighter(ThemeBackend.text, 1.05) : ThemeBackend.text)

            property real targetWidth: (root.showIcon || root.showName) ? implicitWidth : 0
            width: targetWidth
            Behavior on width { NumberAnimation { duration: 480; easing.type: Easing.OutQuint } }

            opacity: initAnimTrigger ? 1.0 : 0.0
            transform: Translate { y: wifiPill.initAnimTrigger ? 0 : s(15); Behavior on y { NumberAnimation { duration: 620; easing.type: Easing.OutQuint } } }
            Behavior on opacity { NumberAnimation { duration: 400; easing.type: Easing.OutCubic } }

            onClicked: if (!root.isPreview) Quickshell.execDetached(["bash", "-c", Caching.payloadDir + "/scripts/qs_manager.sh toggle network wifi"])
        }
    }

    Component {
        id: sideButtonComponent

        ClickButton {
            width: s(root.isCompact ? 28 : 30)
            height: (root.showIcon || root.showName) ? s(root.isCompact ? 28 : 30) : 0
            cornerRadius: Math.max(0, ThemeBackend.borderRadius - s(2))
            horizontalPadding: 0
            buttonIcon: root.showIcon ? root.effectiveIcon : ""
            iconFontSize: s(root.isCompact ? 14 : 15)
            buttonText: (!root.showIcon && root.showName) ? (root.effectiveName.length > 3 ? root.effectiveName.substring(0, 3) : root.effectiveName) : ""
            textFontSize: s(root.isCompact ? 9 : 10)
            accentColor: root.isActive ? (root.isCompact ? Qt.lighter(ThemeBackend.blue, 1.08) : ThemeBackend.blue) : (root.isCompact ? Qt.lighter(ThemeBackend.surface0, 1.18) : ThemeBackend.surface0)
            textColor: root.isActive ? ThemeBackend.base : (root.isCompact ? Qt.lighter(ThemeBackend.text, 1.05) : ThemeBackend.text)
            onClicked: if (!root.isPreview) Quickshell.execDetached(["bash", "-c", Caching.payloadDir + "/scripts/qs_manager.sh toggle network wifi"])
        }
    }
}
