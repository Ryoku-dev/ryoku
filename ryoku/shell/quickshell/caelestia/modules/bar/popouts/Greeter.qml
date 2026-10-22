pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import QtQuick.Effects
import QtMultimedia
import Quickshell
import Quickshell.Wayland
import M3Shapes
import Caelestia.Config
import caelestia.components
import caelestia.components.images
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property PopoutState popouts

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    readonly property string mediaPath: GreeterService.activeMedia

    readonly property int previewSize: Math.round(Tokens.sizes.bar.windowPreviewSize * scaleOffset)

    property Item current: one
    property bool completed: false

    implicitWidth: previewSize
    implicitHeight: previewSize
    width: implicitWidth
    height: implicitHeight

    Component.onCompleted: {
        GreeterService.activePopoutCount++;
        if (mediaPath) {
            one.setPath(mediaPath);
            one.maskRadius = one.maxRadius;
            current = one;
            completed = true;
        }
    }

    Component.onDestruction: {
        GreeterService.activePopoutCount = Math.max(0, GreeterService.activePopoutCount - 1);
    }

    onMediaPathChanged: {
        if (!mediaPath) return;
        if (!completed) {
            one.setPath(mediaPath);
            one.maskRadius = one.maxRadius;
            current = one;
            completed = true;
            return;
        }
        if (current === one) {
            two.setPath(mediaPath);
        } else {
            one.setPath(mediaPath);
        }
    }

    Item {
        id: clipRect

        width: root.previewSize
        height: root.previewSize
        implicitWidth: root.previewSize
        implicitHeight: root.previewSize
        anchors.centerIn: parent

        layer.enabled: true
        layer.format: ShaderEffectSource.RGBA
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: roundedCornerMask
        }

        MediaItem {
            id: one
        }

        MediaItem {
            id: two
        }
    }

    Item {
        id: roundedCornerMaskWrapper

        anchors.fill: clipRect
        visible: false

        Rectangle {
            anchors.fill: parent
            radius: Tokens.rounding.large * root.scaleOffset
            color: "white"
        }
    }

    ShaderEffectSource {
        id: roundedCornerMask

        sourceItem: roundedCornerMaskWrapper
        anchors.fill: clipRect
        hideSource: true
        visible: false
    }

    component MediaItem: Item {
        id: mediaItem

        property string currentPath: ""
        readonly property bool isVideo: Images.isVideo(currentPath)
        readonly property bool isAnimated: Images.isAnimated(currentPath)
        readonly property real maxRadius: Math.sqrt(width * width + height * height)
        property real maskRadius: 0
        readonly property var shapes: [
            MaterialShape.Circle, MaterialShape.Square, MaterialShape.Diamond,
            MaterialShape.ClamShell, MaterialShape.Pentagon, MaterialShape.Gem,
            MaterialShape.Clover4Leaf, MaterialShape.SoftBurst, MaterialShape.Cookie6Sided
        ]
        property int currentShape: MaterialShape.Circle
        readonly property bool needsMask: mediaItem.z === 1 && maskAnim.running

        function setPath(p: string): void {
            if (currentPath === p) {
                root.current = mediaItem;
                return;
            }
            currentPath = p;
            Qt.callLater(() => {
                if (mediaItem.isAnimated) {
                    if (animImg.status === Image.Ready) root.current = mediaItem;
                } else if (mediaItem.isVideo) {
                    if (videoPlayer.playing) root.current = mediaItem;
                } else {
                    if (staticImg.status === Image.Ready) root.current = mediaItem;
                }
            });
        }

        visible: (root.current === mediaItem) || (mediaItem.z === 0 && (root.current as MediaItem)?.needsMask === true)
        anchors.fill: parent
        z: root.current === mediaItem ? 1 : 0

        onZChanged: {
            if (z === 1) {
                if (!root.completed) {
                    maskRadius = maxRadius;
                } else {
                    maskRadius = 0;
                    maskAnim.restart();
                }
            } else {
                maskRadius = 0;
                currentShape = shapes[Math.floor(Math.random() * shapes.length)];
            }
        }

        Item {
            id: maskWrapper

            anchors.fill: parent
            visible: mediaItem.needsMask

            MaterialShape {
                anchors.centerIn: parent
                width: 2000
                height: 2000
                shape: mediaItem.currentShape
                color: "white"
                scale: mediaItem.maxRadius > 0 ? (mediaItem.maskRadius * 2) / 2000 : 0
            }
        }

        ShaderEffectSource {
            id: maskSourceItem

            sourceItem: maskWrapper
            anchors.fill: parent
            hideSource: true
            visible: false
            live: mediaItem.needsMask
        }

        Item {
            id: contentItem

            anchors.fill: parent
            layer.enabled: mediaItem.needsMask
            layer.format: ShaderEffectSource.RGBA
            layer.effect: MultiEffect {
                maskEnabled: mediaItem.needsMask
                maskSource: maskSourceItem
            }

            AnimatedImage {
                id: animImg

                anchors.fill: parent
                cache: false
                asynchronous: true
                fillMode: Image.PreserveAspectCrop
                source: (mediaItem.isAnimated && mediaItem.currentPath) ? (mediaItem.currentPath.startsWith("file:") || mediaItem.currentPath.startsWith("qrc:") ? mediaItem.currentPath : "file://" + mediaItem.currentPath) : ""
                visible: mediaItem.isAnimated && mediaItem.currentPath !== ""
                playing: true

                onSourceChanged: playing = true
                onStatusChanged: {
                    if (status === Image.Ready && mediaItem.isAnimated) {
                        root.current = mediaItem;
                    }
                }
            }

            CachingImage {
                id: staticImg

                anchors.fill: parent
                path: (!mediaItem.isAnimated && !mediaItem.isVideo) ? mediaItem.currentPath : ""
                visible: !mediaItem.isAnimated && !mediaItem.isVideo && mediaItem.currentPath !== ""
                fillMode: Image.PreserveAspectCrop
                onStatusChanged: {
                    if (status === Image.Ready && !mediaItem.isAnimated && !mediaItem.isVideo) {
                        root.current = mediaItem;
                    }
                }
            }

            CachingVideo {
                id: videoPlayer

                anchors.fill: parent
                path: mediaItem.isVideo ? mediaItem.currentPath : ""
                visible: mediaItem.isVideo && mediaItem.currentPath !== ""
                fillMode: VideoOutput.PreserveAspectCrop
                onPlayingChanged: {
                    if (playing && mediaItem.isVideo) {
                        root.current = mediaItem;
                    }
                }
            }
        }

        NumberAnimation {
            id: maskAnim

            target: mediaItem
            property: "maskRadius"
            from: 0
            to: mediaItem.maxRadius
            duration: 1800
            easing.type: Easing.OutCubic
        }
    }
}
