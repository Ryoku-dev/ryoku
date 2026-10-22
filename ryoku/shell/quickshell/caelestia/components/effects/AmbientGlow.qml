pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Widgets
import Caelestia
import Caelestia.Config
import Caelestia.Images
import Caelestia.Services
import caelestia.components
import caelestia.components.images
import caelestia.services
import caelestia.utils

Item {
    id: root

    property string address: ""
    property string activeAddress: ""
    property url fallbackIcon: ""
    property color fallbackColor: Colours.palette.m3primary
    property real sourceAspect: 16 / 9
    property real glowScaleX: 2.0
    property real glowScaleY: 2.0
    property int blurMax: 96
    property real glowOpacity: GlobalConfig.appearance.ambientOpacity
    property real saturation: 1.0
    property real brightness: 0.25
    property real contrast: 0.2
    property real radius: Tokens.rounding.medium
    property bool deform: true
    property bool active: GlobalConfig.appearance.ambientColor && !Colours.light
    property real bloomProgress: 0.0
    property bool _thumbExists: root.thumbPath ? IUtils.fileExists(root.thumbPath) : false

    readonly property string thumbPath: root.activeAddress ? `${Paths.runtimeDir}/caelestia/window-thumbs/${root.activeAddress.startsWith("0x") ? root.activeAddress.slice(2) : root.activeAddress}.png` : ""
    readonly property real fitted: root.sourceAspect > (root.width / Math.max(1, root.height)) ? root.width / root.sourceAspect : root.height
    // shortcut: the KDE window stream is gone, so the glow comes from the thumbnail only.
    readonly property bool hasLiveStream: false

    function triggerSwitch(): void {
        if (!root.active || !root.address) {
            switchAnim.stop();
            bloomInAnim.stop();
            root.bloomProgress = 0.0;
            root.activeAddress = "";
            return;
        }

        if (root.activeAddress === "") {
            root.activeAddress = root.address;
            root.bloomProgress = 0.0;
            bloomInAnim.restart();
            return;
        }

        if (root.activeAddress !== root.address) {
            bloomInAnim.stop();
            switchAnim.restart();
        }
    }

    onAddressChanged: root.triggerSwitch()
    onActiveChanged: root.triggerSwitch()

    opacity: root.active ? 1 : 0
    visible: opacity > 0.01

    Component.onCompleted: {
        root.triggerSwitch();
    }

    Behavior on opacity {
        Anim {
            type: Anim.DefaultEffects
        }
    }

    SequentialAnimation {
        id: switchAnim

        NumberAnimation {
            target: root
            property: "bloomProgress"
            to: 0.0
            duration: 350
            easing.type: Easing.OutQuad
        }

        ScriptAction {
            script: {
                root.activeAddress = root.address;
            }
        }

        NumberAnimation {
            target: root
            property: "bloomProgress"
            to: 1.0
            duration: 1350
            easing.type: Easing.OutCubic
        }
    }

    NumberAnimation {
        id: bloomInAnim

        target: root
        property: "bloomProgress"
        from: 0.0
        to: 1.0
        duration: 1350
        easing.type: Easing.OutCubic
    }

    Item {
        id: glowContainer

        anchors.fill: parent
        opacity: root.glowOpacity * root.bloomProgress
        transform: Scale {
            origin.x: glowContainer.width / 2
            origin.y: glowContainer.height / 2
            xScale: 1.0 + (root.glowScaleX - 1.0) * root.bloomProgress
            yScale: 1.0 + (root.glowScaleY - 1.0) * root.bloomProgress
        }

        Item {
            id: liveGlowItem

            anchors.fill: parent
            visible: root.hasLiveStream
            layer.enabled: visible
            layer.effect: MultiEffect {
                blurEnabled: true
                blur: 1.0
                blurMax: root.blurMax
                saturation: root.saturation
                brightness: root.brightness
                contrast: root.contrast
            }
        }

        Item {
            id: thumbGlowItem

            anchors.fill: parent
            visible: !liveGlowItem.visible && root._thumbExists && thumbImage.status === Image.Ready
            layer.enabled: visible
            layer.effect: MultiEffect {
                blurEnabled: true
                blur: 1.0
                blurMax: root.blurMax
                saturation: root.saturation
                brightness: root.brightness
                contrast: root.contrast
            }

            CachingImage {
                id: thumbImage

                anchors.fill: root.deform ? parent : undefined
                anchors.centerIn: root.deform ? undefined : parent
                width: root.deform ? parent.width : root.fitted * root.sourceAspect
                height: root.deform ? parent.height : root.fitted
                path: root._thumbExists ? root.thumbPath : ""
                fillMode: root.deform ? Image.Stretch : Image.PreserveAspectFit
                asynchronous: true
            }
        }

        Item {
            id: fallbackGlowItem

            anchors.fill: parent
            visible: !liveGlowItem.visible && !thumbGlowItem.visible
            layer.enabled: visible
            layer.effect: MultiEffect {
                blurEnabled: true
                blur: 1.0
                blurMax: root.blurMax
            }

            Rectangle {
                anchors.fill: parent
                radius: root.radius
                color: root.fallbackColor
            }
        }
    }
}
