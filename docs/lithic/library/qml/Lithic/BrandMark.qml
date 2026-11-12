import QtQuick
import QtQuick.Templates as T

T.AbstractButton {
    id: root

    property real markSize: LithicTokens.size.brandMark
    property string previewState: ""
    // the host's own mark in onSel, with at most one jasper accent; null draws Ryoport's octagon
    property Component mark: null
    readonly property bool forcedHover: previewState === "hover"
    readonly property bool forcedPress: previewState === "pressed"

    implicitWidth: markSize
    implicitHeight: markSize
    activeFocusOnTab: true
    hoverEnabled: true
    Accessible.name: Lithic.tr("Home")
    Accessible.role: Accessible.Button
    opacity: enabled ? 1 : LithicTokens.alpha.disabled

    background: CutRect {
        id: brandSurface
        fill: root.hovered || root.forcedHover
              ? Lithic.hoverFill(Lithic.sel, 1) : Lithic.sel
        cut: Math.min(width, height) * LithicTokens.size.brandCutRatio
        ChiselColorBehavior on fill {
            duration: root.hovered || root.forcedHover
                      ? LithicTokens.motion.hoverIn
                      : LithicTokens.motion.hoverOut
        }

        FocusRing {
            x: -outset
            y: -outset
            width: parent.width + outset * 2
            height: parent.height + outset * 2
            keyboardFocusVisible: root.visualFocus || root.previewState === "focus"
            cut: brandSurface.cut
        }
    }

    contentItem: Item {
        y: root.down || root.forcedPress ? Lithic.pressSink : 0

        // the host's mark fits inside the square, inset by its cut
        Loader {
            readonly property real inset: Math.min(root.width, root.height) * LithicTokens.size.brandCutRatio
            x: inset
            y: inset
            width: Math.max(0, parent.width - inset * 2)
            height: Math.max(0, parent.height - inset * 2)
            active: root.mark !== null
            sourceComponent: root.mark
        }

        Canvas {
            id: octagon
            anchors.fill: parent
            visible: root.mark === null
            antialiasing: true

            onWidthChanged: requestPaint()
            onHeightChanged: requestPaint()
            Connections {
                target: Lithic
                function onOnSelChanged() { octagon.requestPaint() }
                function onJasperChanged() { octagon.requestPaint() }
            }

            onPaint: {
                const context = getContext("2d")
                context.reset()
                const side = Math.min(width, height)
                const centerX = width / 2
                const centerY = height / 2
                const apothem = side * LithicTokens.size.brandOctagonApothemRatio
                const diagonal = apothem * (Math.SQRT2 - 1)

                context.strokeStyle = Lithic.onSel
                context.lineWidth = LithicTokens.size.brandOutline
                context.lineJoin = "miter"
                context.beginPath()
                context.moveTo(centerX - diagonal, centerY - apothem)
                context.lineTo(centerX + diagonal, centerY - apothem)
                context.lineTo(centerX + apothem, centerY - diagonal)
                context.lineTo(centerX + apothem, centerY + diagonal)
                context.lineTo(centerX + diagonal, centerY + apothem)
                context.lineTo(centerX - diagonal, centerY + apothem)
                context.lineTo(centerX - apothem, centerY + diagonal)
                context.lineTo(centerX - apothem, centerY - diagonal)
                context.closePath()
                context.stroke()

                const pointRadius = side * LithicTokens.size.brandPointRatio
                context.fillStyle = Lithic.onSel
                context.fillRect(centerX - pointRadius, centerY - pointRadius,
                                 pointRadius * 2, pointRadius * 2)

                context.fillStyle = Lithic.jasper
                context.beginPath()
                context.moveTo(centerX + side * LithicTokens.size.brandPointerNearRatio,
                               centerY + side * LithicTokens.size.brandPointerNearRatio)
                context.lineTo(centerX + side * LithicTokens.size.brandPointerFarRatio,
                               centerY + side * LithicTokens.size.brandPointerMidRatio)
                context.lineTo(centerX + side * LithicTokens.size.brandPointerMidRatio,
                               centerY + side * LithicTokens.size.brandPointerFarRatio)
                context.closePath()
                context.fill()
            }
        }
    }
}
