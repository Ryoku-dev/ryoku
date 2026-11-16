import QtQuick
import Lithic

AnimatedImage {
    id: root
    property bool live: false
    sourceSize.width: Math.max(1, Math.ceil(root.width * 2))
    sourceSize.height: Math.max(1, Math.ceil(root.height * 2))
    playing: root.live && root.visible && !Lithic.reduceMotion
    onPlayingChanged: if (!playing) currentFrame = 0
    fillMode: Image.PreserveAspectCrop
    asynchronous: true
    cache: false
    smooth: true
    mipmap: true
    readonly property bool ready: !source.toString() || status === Image.Ready
}
