pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import shell.services
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane

    readonly property bool outputsReported: {
        const windows = Wm.windows || [];
        for (let i = 0; i < windows.length; ++i) {
            if (String(windows[i].output || "").length > 0)
                return true;
        }
        return false;
    }
    readonly property var focused: {
        const window = Wm.focusedWindow;
        if (!window)
            return null;
        if (!outputsReported || String(window.output || "") === scene.screenName)
            return window;
        return null;
    }
    readonly property string title: focused
        ? String(focused.title || focused.appId || "Window") : ""
    readonly property url iconSource: focused ? Icons.path(focused.appId, true) : ""
    readonly property real iconSize: Math.round(16 * K.Style.scale)
    readonly property real contentGap: Math.round(6 * K.Style.scale)
    readonly property real layoutPreferredTextWidth: Math.min(
        titleMeasure.implicitWidth, Math.max(0, Number(S.Prefs.windowWidth) || 240))
    readonly property real layoutFixedWidth: 2 * K.Style.hitPad
        + iconSize + contentGap
    readonly property real titleWidth: Math.max(0,
        Math.min(layoutPreferredTextWidth, scene.textRoom))

    property bool present: focused !== null

    implicitWidth: present ? layoutFixedWidth + titleWidth : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Component.onCompleted: scene.registerTextModule("window", root)
    Component.onDestruction: scene.unregisterTextModule("window", root)

    K.Label {
        id: titleMeasure
        visible: false
        text: root.title
        role: "bar"
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: root.contentGap
        scale: press.pressScale

        Item {
            width: root.iconSize
            height: root.iconSize

            Image {
                anchors.fill: parent
                visible: root.iconSource.toString().length > 0
                source: root.iconSource
                sourceSize: {
                    const px = Math.max(1,
                        Math.round(root.iconSize * Screen.devicePixelRatio * 2));
                    return Qt.size(px, px);
                }
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                smooth: true
                mipmap: true
            }

            K.Glyph {
                anchors.centerIn: parent
                visible: root.iconSource.toString().length === 0
                name: "web_asset"
                size: root.iconSize
                tone: "dim"
            }
        }

        K.Label {
            width: root.titleWidth
            anchors.verticalCenter: parent.verticalCenter
            text: root.title
            role: "bar"
            tone: "ink"
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleOverview()
        onRightClicked: root.scene.toggleOverview()
    }

    K.Hint {
        target: root
        text: root.title
        shown: root.present && press.hovered
    }
}