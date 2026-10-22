import QtQuick
import Quickshell
import Caelestia.Config

Region {
    id: root

    required property Item target
    required property Item contentItem
    property string vAnchor: (target && target["vAnchor"] !== undefined) ? target["vAnchor"] : "bottom" 
    property string hAnchor: (target && target["hAnchor"] !== undefined) ? target["hAnchor"] : "right"
    property real offsetScale: (target && target["offsetScale"] !== undefined) ? target["offsetScale"] : 0
    
    property real blurOffsetTop: 0
    property real blurOffsetBottom: 0
    property real blurOffsetLeft: 0
    property real blurOffsetRight: 0
    property matrix4x4 deformMatrix

    property BlurOffsets offsets: BlurOffsets {
        target: root.target
        contentItem: root.contentItem
        vAnchor: root.vAnchor
        hAnchor: root.hAnchor
        offsetScale: root.offsetScale
        blurOffsetTop: root.blurOffsetTop
        blurOffsetBottom: root.blurOffsetBottom
        blurOffsetLeft: root.blurOffsetLeft
        blurOffsetRight: root.blurOffsetRight
        deformMatrix: root.deformMatrix
    }

    BlurBodies {
        bX: root.offsets.bX
        bY: root.offsets.bY
        bW: root.offsets.bW
        bH: root.offsets.bH
        inLeft: root.offsets.inLeft
        inRight: root.offsets.inRight
        inTop: root.offsets.inTop
        inBottom: root.offsets.inBottom
    }

    BlurCorners {
        vAnchor: root.vAnchor
        hAnchor: root.hAnchor
        blurQuality: root.offsets.blurSettings.blurQuality
        inLeft: root.offsets.inLeft
        inRight: root.offsets.inRight
        inTop: root.offsets.inTop
        inBottom: root.offsets.inBottom
        rTop: root.offsets.rTop
        rBottom: root.offsets.rBottom
        rLeft: root.offsets.rLeft
        rRight: root.offsets.rRight
    }

    // Pure native blur fallback: a static rectangle covering the widget's layout area,
    // explicitly avoiding 'item: root.target' so we don't inherit its deformMatrix.
    Region {
        x: (!GlobalConfig.appearance.blurMask && root.offsets.isActive) ? root.offsets.tX : 0
        y: (!GlobalConfig.appearance.blurMask && root.offsets.isActive) ? root.offsets.tY : 0
        width: (!GlobalConfig.appearance.blurMask && root.offsets.isActive) ? root.offsets.tW : 0
        height: (!GlobalConfig.appearance.blurMask && root.offsets.isActive) ? root.offsets.tH : 0
    }
}
