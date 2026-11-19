pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import Ryoku.Ui.Singletons
import "../look"
import Quickshell
import ".."
import "parts"

// Beautify's edits, floating beside the lifted shot in the design's own idiom:
// Lithic's numbered specification sheet flush on the right edge, Material's
// floating side sheet with a carousel of looks and an extended FAB, Adwaita's
// sidebar card with a header bar and boxed lists, Fluent's acrylic pane with a
// CommandBar and SettingsCards. The content is the same in all four; the parts
// draw it in each. It scrolls inside once it is taller than maxHeight.
Item {
    id: panel

    required property var studio
    property real u: 1
    property real maxHeight: 800
    // where Material's container transform starts: the dock's Beautify button
    property var entryRect: null
    property bool shown: false

    // the gap to the output's right edge: Lithic sits flush, the others float
    readonly property real edgeMargin: Math.round((Look.lithic ? 0 : Look.material ? 16 : 12) * panel.u)
    readonly property real pad: Math.round((Look.lithic ? 16 : Look.material ? 24 : 12) * panel.u)

    width: Math.round((Look.lithic ? 320 : 340) * panel.u)
    height: Math.min(panel.maxHeight, header.height + content.height + panel.bodyPadTop + panel.bodyPadBottom + footer.height)
    readonly property real bodyPadTop: Math.round((Look.lithic ? 14 : Look.material ? 4 : Look.adwaita ? 6 : 4) * panel.u)
    readonly property real bodyPadBottom: Math.round((Look.lithic ? 18 : Look.material ? 8 : 12) * panel.u)

    readonly property var kinds: [
        { "k": "preset", "l": I18n.tr("Gradient") },
        { "k": "solid", "l": I18n.tr("Solid") },
        { "k": "image", "l": I18n.tr("Image") },
        { "k": "none", "l": I18n.tr("None") }
    ]
    readonly property var friendly: ({ "auto": I18n.tr("Automatic"), "1:1": I18n.tr("Square"),
        "16:9": I18n.tr("Widescreen 16:9"), "9:16": I18n.tr("Portrait 9:16") })
    readonly property var ratios: panel.studio ? panel.studio.ratioRow.map(o => Object.assign({ "f": panel.friendly[o.k] || o.l }, o)) : []
    readonly property var socials: panel.studio ? panel.studio.socialRow : []

    readonly property string kind: !panel.studio ? "preset" : panel.studio.bgKind === "gradient" ? "preset" : panel.studio.bgKind
    function setKind(k) {
        if (k === "image") {
            panel.studio.bgKind = "image";
            if (!panel.studio.bgImagePath)
                panel.studio.pickImage();
            return;
        }
        // Gradient keeps a custom gradient that is already chosen
        if (k === "preset" && panel.studio.bgKind === "gradient")
            return;
        panel.studio.bgKind = k;
    }
    function signed(v) {
        const n = Math.round(v);
        return (n > 0 ? "+" : "") + n;
    }
    // the watermark's handle, as the composition stamps it
    readonly property string handle: (Quickshell.env("USER") || "user") + "@RyokuArch"

    // ------------------------------------------------------------ motion
    // Lithic slides out of the right edge and lands with one shine; Material
    // grows out of the dock's button (or springs in from the right); Adwaita
    // fades and rises 16 px; Fluent slides 40 px in and fades. Reduced motion
    // crossfades in place.
    property real enter: 0
    Behavior on enter {
        id: enterMotion
        enabled: !Look.reduceMotion
        NumberAnimation {
            duration: enterMotion.targetValue > 0.5
                ? (Look.material ? (panel.morphs ? 460 : 400) : Look.fluent ? 250 : 200)
                : (Look.material && panel.morphs ? 220 : 150)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: enterMotion.targetValue > 0.5
                ? (Look.material ? Look.springDefault : Look.fluent ? Look.fluentIn : Look.adwaita ? Look.enterCurve : Look.decelerate)
                : (Look.fluent ? Look.fluentOut : Look.accelerate)
        }
    }
    property real fade: 0
    Behavior on fade {
        enabled: Look.reduceMotion
        NumberAnimation { duration: 150; easing.type: Easing.OutCubic }
    }
    Component.onCompleted: {
        panel.enter = Qt.binding(() => panel.shown ? 1 : 0);
        panel.fade = Qt.binding(() => panel.shown ? 1 : 0);
    }
    onShownChanged: {
        if (panel.shown && Look.lithic && !Look.reduceMotion)
            shineRun.restart();
    }
    SequentialAnimation {
        id: shineRun
        PropertyAction { target: sheet; property: "shine"; value: -0.3 }
        PauseAnimation { duration: 180 }
        NumberAnimation { target: sheet; property: "shine"; from: -0.3; to: 1.3; duration: 760; easing.type: Easing.OutCubic }
    }

    readonly property real e: Math.max(0, Math.min(1, panel.enter))
    // the studio hands the button over as {x, y, w, h}
    readonly property real entryW: panel.entryRect ? (panel.entryRect.w !== undefined ? panel.entryRect.w : panel.entryRect.width) || 0 : 0
    readonly property real entryH: panel.entryRect ? (panel.entryRect.h !== undefined ? panel.entryRect.h : panel.entryRect.height) || 0 : 0
    readonly property bool morphs: Look.material && !Look.reduceMotion && panel.entryW > 0 && panel.entryH > 0
    // where the panel sits in the studio, whose child (a Loader) holds it
    readonly property point at: panel.parent ? Qt.point(panel.parent.x + panel.x, panel.parent.y + panel.y) : Qt.point(0, 0)
    // the button's rect in the panel's own coordinates
    readonly property rect from: panel.morphs
        ? Qt.rect(panel.entryRect.x - panel.at.x, panel.entryRect.y - panel.at.y, panel.entryW, panel.entryH)
        : Qt.rect(0, 0, panel.width, panel.height)
    // the sheet's rect through Material's container transform
    readonly property rect frame: panel.morphs
        ? Qt.rect(panel.from.x + (0 - panel.from.x) * panel.enter, panel.from.y + (0 - panel.from.y) * panel.enter,
            Math.max(1, panel.from.width + (panel.width - panel.from.width) * panel.enter),
            Math.max(1, panel.from.height + (panel.height - panel.from.height) * panel.enter))
        : Qt.rect(0, 0, panel.width, panel.height)

    visible: Look.reduceMotion ? panel.fade > 0 : panel.enter > 0.001
    enabled: panel.shown
    opacity: Look.reduceMotion ? panel.fade
        : Look.lithic ? 1
        : Look.material ? (panel.morphs ? 1 : Math.min(1, panel.e * 2.5))
        : panel.e
    transform: Translate {
        x: Look.reduceMotion ? 0
            : Look.lithic ? (1 - panel.enter) * panel.width
            : Look.fluent ? (1 - panel.e) * 40 * panel.u
            : Look.material && !panel.morphs ? (1 - panel.enter) * 60 * panel.u
            : 0
        y: Look.reduceMotion ? 0 : Look.adwaita ? (1 - panel.e) * 16 * panel.u : 0
    }

    // nothing under the panel hears a click or a wheel meant for it
    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        acceptedButtons: Qt.AllButtons
        onWheel: (w) => w.accepted = true
    }

    Sheet {
        id: sheet
        u: panel.u
        x: panel.frame.x
        y: panel.frame.y
        width: panel.frame.width
        height: panel.frame.height
        radius: panel.morphs ? (panel.from.height / 2 + (28 * panel.u - panel.from.height / 2) * panel.e) : 28 * panel.u
    }

    // Material's container transform: the content grows inside the sheet, cut to it
    Item {
        id: morphClip
        x: panel.frame.x
        y: panel.frame.y
        width: panel.frame.width
        height: panel.frame.height
        clip: panel.morphs && panel.enter < 0.999

        Item {
            id: chrome
            width: panel.width
            height: panel.height
            // the container transform scales the content with the sheet and fades it
            // in once the sheet has mostly grown
            opacity: panel.morphs ? Math.max(0, Math.min(1, (panel.enter - 0.35) / 0.5)) : 1
            transform: Scale {
                xScale: panel.morphs ? panel.frame.width / panel.width : 1
                yScale: panel.morphs ? panel.frame.width / panel.width : 1
            }

            Header {
                id: header
                width: parent.width
                u: panel.u
                studio: panel.studio
                scrolled: flick.contentY > 1
            }

            Flickable {
                id: flick
                y: header.height
                width: parent.width
                height: panel.height - header.height - footer.height
                contentWidth: width
                contentHeight: content.height + panel.bodyPadTop + panel.bodyPadBottom
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                interactive: contentHeight > height
                QQC.ScrollBar.vertical: QQC.ScrollBar {
                    id: bar
                    policy: flick.contentHeight > flick.height ? QQC.ScrollBar.AsNeeded : QQC.ScrollBar.AlwaysOff
                    width: Math.round(10 * panel.u)
                    rightPadding: Math.round((Look.lithic ? 3 : 3) * panel.u)
                    contentItem: Item {
                        implicitWidth: Math.round((Look.material ? 4 : 3) * panel.u)
                        opacity: bar.pressed ? 0.9 : bar.hovered ? 0.7 : bar.active ? 0.5 : 0.0
                        Behavior on opacity {
                            NumberAnimation { duration: Look.dur(160) }
                        }
                        CutRect {
                            visible: Look.lithic
                            anchors.fill: parent
                            design: "lithic"
                            fill: Look.fg3
                        }
                        Rectangle {
                            visible: !Look.lithic
                            anchors.fill: parent
                            anchors.leftMargin: bar.hovered && (Look.fluent || Look.adwaita) ? -Math.round(2 * panel.u) : 0
                            radius: width / 2
                            color: Look.fg2
                        }
                    }
                }

                Column {
                    id: content
                    x: panel.pad
                    y: panel.bodyPadTop
                    width: panel.width - 2 * panel.pad
                    spacing: Math.round((Look.lithic ? 24 : Look.material ? 22 : Look.adwaita ? 18 : 20) * panel.u)

                    // 01 look
                    Section {
                        width: parent.width
                        u: panel.u
                        number: 1
                        title: I18n.tr("Look")
                        boxed: false
                        Looks {
                            width: parent.width
                            u: panel.u
                            studio: panel.studio
                        }
                    }

                    // 02 background
                    Section {
                        width: parent.width
                        u: panel.u
                        number: 2
                        title: I18n.tr("Background")
                        Picker {
                            u: panel.u
                            menus: menus
                            label: I18n.tr("Type")
                            glyph: "image"
                            options: panel.kinds
                            current: panel.kind
                            onPicked: (k) => panel.setKind(k)
                        }
                        Field {
                            visible: panel.kind === "preset"
                            u: panel.u
                            block: Gradients {
                                width: parent ? parent.width : 0
                                u: panel.u
                                studio: panel.studio
                            }
                        }
                        Field {
                            visible: panel.studio && panel.studio.bgKind === "gradient"
                            u: panel.u
                            block: CustomGradient {
                                width: parent ? parent.width : 0
                                u: panel.u
                                studio: panel.studio
                            }
                        }
                        SliderRow {
                            visible: panel.studio && panel.studio.bgKind === "gradient"
                            u: panel.u
                            studio: panel.studio
                            key: "bgGradAngle"
                            label: I18n.tr("Angle")
                            glyph: "compass"
                            to: 360
                            suffix: "\u00b0"
                        }
                        Field {
                            visible: panel.kind === "solid"
                            u: panel.u
                            block: Swatches {
                                width: parent ? parent.width : 0
                                u: panel.u
                                colors: panel.studio ? panel.studio.palette : []
                                current: panel.studio ? panel.studio.bgSolid : "white"
                                onPicked: (c) => panel.studio.bgSolid = c
                            }
                        }
                        ImageRow {
                            visible: panel.kind === "image"
                            u: panel.u
                            path: panel.studio ? panel.studio.bgImagePath : ""
                            onChoose: panel.studio.pickImage()
                        }
                    }

                    // 03 frame
                    Section {
                        width: parent.width
                        u: panel.u
                        number: 3
                        title: I18n.tr("Frame")
                        SliderRow {
                            u: panel.u
                            studio: panel.studio
                            key: "padding"
                            label: I18n.tr("Padding")
                            glyph: "padding"
                            to: 240
                        }
                        SliderRow {
                            u: panel.u
                            studio: panel.studio
                            key: "roundness"
                            label: I18n.tr("Corners")
                            glyph: "corners"
                            to: 80
                        }
                        SliderRow {
                            u: panel.u
                            studio: panel.studio
                            key: "borderW"
                            label: I18n.tr("Border")
                            glyph: "rect"
                            to: 16
                        }
                        Field {
                            visible: panel.studio && panel.studio.borderW > 0
                            u: panel.u
                            block: Swatches {
                                width: parent ? parent.width : 0
                                u: panel.u
                                colors: panel.studio ? panel.studio.palette : []
                                current: panel.studio ? panel.studio.borderColor : "white"
                                onPicked: (c) => panel.studio.borderColor = c
                            }
                        }
                    }

                    // 04 shadow
                    Section {
                        width: parent.width
                        u: panel.u
                        number: 4
                        title: I18n.tr("Shadow")
                        SliderRow {
                            u: panel.u
                            studio: panel.studio
                            key: "shadowStrength"
                            label: I18n.tr("Strength")
                            glyph: "shadow"
                            suffix: "%"
                        }
                        Expander {
                            u: panel.u
                            title: I18n.tr("Shadow details")
                            glyph: "compass"
                            summary: panel.studio ? Math.round(panel.studio.shadowBlur) + " \u00b7 " + Math.round(panel.studio.shadowDist)
                                + " px \u00b7 " + Math.round(panel.studio.shadowAngle) + "\u00b0" : ""
                            SliderRow {
                                u: panel.u
                                bare: true
                                studio: panel.studio
                                key: "shadowBlur"
                                label: I18n.tr("Softness")
                            }
                            SliderRow {
                                u: panel.u
                                bare: true
                                studio: panel.studio
                                key: "shadowDist"
                                label: I18n.tr("Distance")
                                to: 80
                            }
                            DialRow {
                                u: panel.u
                                bare: true
                                studio: panel.studio
                                title: I18n.tr("Direction")
                            }
                        }
                    }

                    // 05 size
                    Section {
                        width: parent.width
                        u: panel.u
                        number: 5
                        title: I18n.tr("Size")
                        Picker {
                            u: panel.u
                            menus: menus
                            label: I18n.tr("Format")
                            glyph: "aspect"
                            options: panel.ratios
                            extra: panel.socials
                            extraTitle: I18n.tr("For")
                            current: panel.studio ? panel.studio.ratioKey : "auto"
                            onPicked: (k) => panel.studio.ratioKey = k
                        }
                    }

                    // 06 adjust
                    Section {
                        width: parent.width
                        u: panel.u
                        number: 6
                        title: I18n.tr("Adjust")
                        Expander {
                            u: panel.u
                            title: I18n.tr("Brightness, contrast, saturation")
                            glyph: "sun"
                            summary: panel.studio ? panel.signed(panel.studio.adjBright) + " \u00b7 " + panel.signed(panel.studio.adjContrast)
                                + " \u00b7 " + panel.signed(panel.studio.adjSat) : ""
                            SliderRow {
                                u: panel.u
                                bare: true
                                studio: panel.studio
                                key: "adjBright"
                                label: I18n.tr("Brightness")
                                bipolar: true
                                from: -100
                            }
                            SliderRow {
                                u: panel.u
                                bare: true
                                studio: panel.studio
                                key: "adjContrast"
                                label: I18n.tr("Contrast")
                                bipolar: true
                                from: -100
                            }
                            SliderRow {
                                u: panel.u
                                bare: true
                                studio: panel.studio
                                key: "adjSat"
                                label: I18n.tr("Saturation")
                                bipolar: true
                                from: -100
                            }
                        }
                    }

                    // 07 output
                    Section {
                        width: parent.width
                        u: panel.u
                        number: 7
                        title: I18n.tr("Output")
                        SwitchRow {
                            u: panel.u
                            title: I18n.tr("Watermark")
                            subtitle: (panel.studio && panel.studio.mark ? panel.studio.mark + "  " : "") + panel.handle
                            glyph: "stamp"
                            checked: panel.studio ? panel.studio.watermark : false
                            onToggled: (v) => panel.studio.watermark = v
                        }
                        SwitchRow {
                            u: panel.u
                            title: I18n.tr("Double resolution (HD)")
                            glyph: "hd"
                            checked: panel.studio ? panel.studio.hd : false
                            onToggled: (v) => panel.studio.hd = v
                        }
                    }

                    // Use every time, and Adwaita's Save; Lithic keeps both in its foot
                    Section {
                        visible: !Look.lithic
                        width: parent.width
                        u: panel.u
                        SwitchRow {
                            u: panel.u
                            title: I18n.tr("Use every time")
                            subtitle: I18n.tr("Copies with your Beautify style")
                            glyph: "repeat"
                            checked: panel.studio ? panel.studio.always : false
                            onToggled: (v) => panel.studio.setAlways(v)
                        }
                        ButtonRow {
                            visible: Look.adwaita
                            u: panel.u
                            icon: "save"
                            text: I18n.tr("Save")
                            enabled: panel.studio && !panel.studio.busy
                            onClicked: panel.studio.save()
                        }
                    }
                }
            }

            // where the body runs on under the head or the foot, it fades into the sheet
            Rectangle {
                visible: !Look.adwaita
                x: Look.lithic ? Math.max(1, Math.round(panel.u)) : Math.round(12 * panel.u)
                y: flick.y
                width: parent.width - 2 * x
                height: Math.round(18 * panel.u)
                opacity: Math.min(1, flick.contentY / (18 * panel.u))
                gradient: Gradient {
                    GradientStop { position: 0; color: sheet.fill }
                    GradientStop { position: 1; color: Lithic.alpha(sheet.fill, 0) }
                }
            }
            Rectangle {
                visible: !Look.material
                x: Look.lithic ? Math.max(1, Math.round(panel.u)) : Math.round(12 * panel.u)
                y: flick.y + flick.height - height
                width: parent.width - 2 * x
                height: Math.round(24 * panel.u)
                opacity: Math.min(1, Math.max(0, flick.contentHeight - flick.height - flick.contentY) / (24 * panel.u))
                gradient: Gradient {
                    GradientStop { position: 0; color: Lithic.alpha(sheet.fill, 0) }
                    GradientStop { position: 1; color: sheet.fill }
                }
            }

            Footer {
                id: footer
                y: panel.height - height
                width: parent.width
                u: panel.u
                studio: panel.studio
            }
        }
    }

    PopMenu {
        id: menus
        x: -panel.at.x
        y: -panel.at.y
        width: panel.studio ? panel.studio.width : panel.width
        height: panel.studio ? panel.studio.height : panel.height
        z: 100
        u: panel.u
        studio: panel.studio
    }
}
