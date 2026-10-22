pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import QtMultimedia
import Quickshell
import M3Shapes
import Caelestia.Config
import caelestia.components
import caelestia.components.filedialog
import caelestia.components.images
import caelestia.services
import caelestia.utils

Item {
    id: root

    property string source: Wallpapers.current
    property Item current: one
    property bool completed
    property bool skipTransition: false
    property var screen: null

    readonly property bool shown: root.current ? (root.current.shown || root.source === "") : false

    function isVideo(path: string): bool {
        if (!path)
            return false;
        const ext = path.split('.').pop().toLowerCase();
        return ["mp4", "webm", "mkv", "avi", "mov", "wmv", "flv"].includes(ext);
    }

    function releaseHidden(): void {
        if (!current || (current.videoPath !== source && current.imagePath !== source))
            return;
        for (const img of [one, two])
            if (img !== current)
                img.videoPath = "";
    }

    onSourceChanged: {
        if (!source)
            current = null;
        else if (current === one) {
            two.screen = screen;
            two.update();
        } else {
            one.screen = screen;
            one.update();
        }
    }
    Component.onCompleted: {
        if (source)
            Qt.callLater(() => {
                one.screen = screen;
                Qt.callLater(() => one.update());
                completed = true;
                Logger.mark("wallpaper-ready");
            });
    }

    Timer {
        id: slideshowTimer

        interval: Math.max(1, Math.round(Config.background.slideshowInterval * 60)) * 60 * 1000
        running: Config.background.slideshowEnabled && Config.background.wallpaperEnabled && root.screen && root.screen.name === Quickshell.screens[0].name
        repeat: true
        onTriggered: {
            if (Config.background.slideshowRandom) {
                Wallpapers.setRandom();
            } else {
                let idx = -1;
                for (let i = 0; i < Wallpapers.list.length; i++) {
                    if (Wallpapers.list[i].path === root.source) {
                        idx = i;
                        break;
                    }
                }
                if (idx !== -1 && Wallpapers.list.length > 0) {
                    let nextIdx = (idx + 1) % Wallpapers.list.length;
                    Wallpapers.setWallpaper(Wallpapers.list[nextIdx].path);
                } else if (Wallpapers.list.length > 0) {
                    Wallpapers.setWallpaper(Wallpapers.list[0].path);
                }
            }
        }
    }
    Loader {
        asynchronous: true
        anchors.fill: parent
        active: root.completed && !root.source
        sourceComponent: StyledRect {
            color: Colours.palette.m3surfaceContainer

            Row {
                anchors.centerIn: parent
                spacing: Tokens.spacing.largeIncreased

                MaterialIcon {
                    text: "sentiment_stressed"
                    color: Colours.palette.m3onSurfaceVariant
                    fontStyle: Tokens.font.icon.builders.extraLarge.scale(5).build()
                }
                Column {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.spacing.small

                    StyledText {
                        text: qsTr("Wallpaper missing?")
                        color: Colours.palette.m3onSurfaceVariant
                        font: Tokens.font.body.builders.large.size(28 * 2).weight(Font.Bold).build()
                    }
                    StyledRect {
                        implicitWidth: selectWallText.implicitWidth + Tokens.padding.extraLargeIncreased
                        implicitHeight: selectWallText.implicitHeight + Tokens.padding.small
                        radius: Tokens.rounding.full
                        color: Colours.palette.m3primary

                        FileDialog {
                            id: dialog

                            title: qsTr("Select a wallpaper")
                            filterLabel: qsTr("Media files")
                            filters: Images.validImageExtensions.concat(Images.validVideoExtensions)
                            onAccepted: path => Wallpapers.setWallpaper(path)
                        }
                        StateLayer {
                            radius: parent.radius
                            color: Colours.palette.m3onPrimary
                            onClicked: dialog.open()
                        }
                        StyledText {
                            id: selectWallText

                            anchors.centerIn: parent
                            text: qsTr("Set it now!")
                            color: Colours.palette.m3onPrimary
                            font: Tokens.font.body.large
                        }
                    }
                }
            }
        }
    }
    Img {
        id: one

        property var screen: null
    }
    Img {
        id: two

        property var screen: null
    }

    component Img: Item {
        id: img

        property string imagePath: ""
        property string videoPath: ""
        // Follows what this layer holds, not root.source: the hidden layer keeps
        // its old content and must not start decoding a new video as well.
        readonly property bool isVideoImage: videoPath !== ""
        property var screen: null
        readonly property bool shown: {
            if (root.source === "")
                return false;
            return img.isVideoImage ? wallpaperVideo.playing : wallpaperImage.status === Image.Ready;
        }
        readonly property real maxRadius: Math.sqrt(width * width + height * height)
        property real maskRadius: root.skipTransition ? maxRadius : 0
        readonly property var shapes: [
            MaterialShape.Circle, MaterialShape.Square, MaterialShape.Diamond,
            MaterialShape.ClamShell, MaterialShape.Pentagon, MaterialShape.Gem,
            MaterialShape.Clover4Leaf, MaterialShape.SoftBurst, MaterialShape.Cookie6Sided
        ]
        property int currentShape: MaterialShape.Circle
        readonly property string currentSchemeName: Colours.showPreview ? Colours.previewScheme : Colours.scheme
        readonly property string currentVariantName: Colours.showPreview ? Colours.previewVariant : Colours.variant
        readonly property bool isDynamicScheme: currentSchemeName.startsWith("dynamic")
        readonly property bool isDynamicMonochrome: isDynamicScheme && currentVariantName === "monochrome"
        readonly property bool needsMask: img.z === 1 && maskAnim.running
        readonly property bool tiled: {
            const m = Config.background.wallpaperFillMode;
            return m === Image.Tile || m === Image.TileVertically || m === Image.TileHorizontally;
        }
        readonly property bool shouldRecolor: Config.background.wallpaperRecolor

        function update(): void {
            this.screen = root.screen;
            const isVideoImage = root.isVideo(root.source);
            if (isVideoImage) {
                if (videoPath === root.source)
                    root.current = this;
                else {
                    imagePath = "";
                    videoPath = root.source;
                }
            } else {
                if (imagePath === root.source)
                    root.current = this;
                else {
                    videoPath = "";
                    imagePath = root.source;
                }
            }
        }

        anchors.fill: parent
        opacity: 1
        scale: 1
        Component.onCompleted: maskRadius = root.skipTransition ? maxRadius : maxRadius
        z: root.current === img ? 1 : 0
        onZChanged: {
            if (z === 1) {
                if (root.skipTransition) {
                    maskRadius = maxRadius;
                    Qt.callLater(root.releaseHidden);
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

            MaterialShape {
                anchors.centerIn: parent
                width: 2000
                height: 2000
                shape: img.currentShape
                color: "white"
                scale: img.maxRadius > 0 ? (img.maskRadius * 2) / 2000 : 0
            }
        }
        ShaderEffectSource {
            id: maskSourceItem

            sourceItem: maskWrapper
            anchors.fill: parent
            hideSource: true
            live: img.needsMask
        }
        Item {
            id: contentItem

            anchors.fill: parent
            layer.enabled: needsMask || Config.background.wallpaperRecolor
            layer.effect: MultiEffect {
                readonly property string currentFlavourName: Colours.showPreview ? Colours.previewFlavour : Colours.flavour

                maskEnabled: img.needsMask
                maskSource: maskSourceItem
                shadowEnabled: img.needsMask
                shadowColor: "black"
                shadowBlur: 1.0
                shadowVerticalOffset: 15
                shadowHorizontalOffset: 5
                saturation: (img.shouldRecolor && img.isDynamicMonochrome) ? -1 : 0
                colorization: img.shouldRecolor ? Config.background.wallpaperRecolorStrength : 0
                colorizationColor: Colours.palette.m3primary
                contrast: (img.shouldRecolor && currentFlavourName === "hard") ? 0.45 : 0.0

                Behavior on saturation { Anim { type: Anim.DefaultEffects } }
                Behavior on colorization { Anim { type: Anim.DefaultEffects } }
                Behavior on contrast { Anim { type: Anim.DefaultEffects } }
                Behavior on colorizationColor {
                    CAnim {}
                }
            }

            CachingAnimatedImage {
                id: wallpaperImage

                anchors.fill: parent
                path: img.imagePath
                visible: !img.isVideoImage && img.imagePath !== ""
                asynchronous: true
                fillMode: Config.background.wallpaperFillMode
                source: img.imagePath || ""
                playing: true
                sourceSize: {
                    if (img.isVideoImage || img.imagePath === "" || img.tiled)
                        return Qt.size(0, 0);
                    const dpr = wallpaperImage.Screen.devicePixelRatio || 1;
                    const edge = Math.ceil(Math.max(wallpaperImage.width, wallpaperImage.height) * dpr * 1.5);
                    return edge > 0 ? Qt.size(edge, 0) : Qt.size(0, 0);
                }
                onStatusChanged: {
                    if (status === Image.Ready && !img.isVideoImage)
                        root.current = img;
                }
            }
            CachingVideo {
                id: wallpaperVideo

                anchors.fill: parent
                path: img.videoPath
                screen: root.screen
                visible: img.isVideoImage && img.videoPath !== ""
                fillMode: Config.background.wallpaperFillMode
                onPlayingChanged: {
                    if (playing && img.isVideoImage)
                        root.current = img;
                }
            }
        }
            Anim {
                id: maskAnim

                target: img
                property: "maskRadius"
                from: 0
                to: img.maxRadius
                type: Anim.Emphasized
                duration: 2500
                onFinished: root.releaseHidden()
            }
    }
}
