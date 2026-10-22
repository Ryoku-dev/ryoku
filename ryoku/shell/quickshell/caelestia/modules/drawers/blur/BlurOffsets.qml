import QtQuick
import QtCore
import Caelestia.Config

Item {
    id: root

    required property Item target
    property string vAnchor: "bottom" 
    property string hAnchor: "right"
    property real offsetScale: 0
    required property Item contentItem
    property real blurOffsetTop: 0
    property real blurOffsetBottom: 0
    property real blurOffsetLeft: 0
    property real blurOffsetRight: 0
    property bool isActive: target && target.visible && target.opacity > 0 && GlobalConfig.appearance.transparency.enabled && GlobalConfig.appearance.blur
    property Settings blurSettings: Settings {
        property int blurQuality: 20

        category: "Blur"
    }
    // The centered deformMatrix from the BlobRect (identity by default)
    // m11 = horizontal scale, m22 = vertical scale, applied around the center of the target
    property matrix4x4 deformMatrix
    property real animScale: 1 - offsetScale
    property real sTop: (vAnchor === "top" ? blurOffsetBottom : blurOffsetTop) * animScale
    property real sBottom: (vAnchor === "top" ? blurOffsetTop : blurOffsetBottom) * animScale
    property real sLeft: (hAnchor === "left" ? blurOffsetRight : blurOffsetLeft) * animScale
    property real sRight: (hAnchor === "left" ? blurOffsetLeft : blurOffsetRight) * animScale
    property real tX: {
        if (!target) return 0;
        let _ = target.x + target.width;
        return target.parent ? target.parent.mapToItem(contentItem, target.x, target.y).x : 0;
    }
    property real tY: {
        if (!target) return 0;
        let _ = target.y + target.height;
        return target.parent ? target.parent.mapToItem(contentItem, target.x, target.y).y : 0;
    }
    property real tW: target ? target.width : 0
    property real tH: target ? target.height : 0
    property bool useMasks: GlobalConfig.appearance.blurMask
    property real dm11: (useMasks && deformMatrix.m11 > 0) ? deformMatrix.m11 : 1
    property real dm22: (useMasks && deformMatrix.m22 > 0) ? deformMatrix.m22 : 1
    property real deformOffsetX: useMasks ? (tW / 2) * (1 - dm11) : 0
    property real deformOffsetY: useMasks ? (tH / 2) * (1 - dm22) : 0
    property real bX: useMasks ? tX + deformOffsetX + sLeft * dm11 : 0
    property real bY: useMasks ? tY + deformOffsetY + sTop * dm22 : 0
    property real bW: (isActive && useMasks) ? Math.max(0, tW * dm11 - sLeft * dm11 - sRight * dm11) : 0
    property real bH: (isActive && useMasks) ? Math.max(0, tH * dm22 - sTop * dm22 - sBottom * dm22) : 0
    property real r: (isActive && useMasks) ? Tokens.rounding.extraLarge * animScale : 0
    property bool isIsland: GlobalConfig.appearance.islands
    property real rTop: (!isIsland && (vAnchor === "top" || vAnchor === "both")) ? 0 : r
    property real rBottom: (!isIsland && (vAnchor === "bottom" || vAnchor === "both")) ? 0 : r
    property real rLeft: (!isIsland && (hAnchor === "left" || hAnchor === "both")) ? 0 : r
    property real rRight: (!isIsland && (hAnchor === "right" || hAnchor === "both")) ? 0 : r
    property real inLeft: useMasks ? bX + rLeft : 0
    property real inRight: useMasks ? bX + bW - rRight : 0
    property real inTop: useMasks ? bY + rTop : 0
    property real inBottom: useMasks ? bY + bH - rBottom : 0
}
