pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import QtQuick.Effects
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

QQC.Popup {
    id: root

    required property real s
    required property bool active
    signal openExtensions()
    signal requestClose()

    readonly property var pluginCards: pluginHost.cards()
    readonly property real preferredWidth: Math.round(300 * root.s)
    // the popup is parented to the session row, so the window overlay bounds it
    readonly property Item host: QQC.Overlay.overlay
    readonly property real hostWidth: root.host && root.host.width > 0 ? root.host.width : root.preferredWidth
    readonly property real hostHeight: root.host && root.host.height > 0 ? root.host.height : root.implicitHeight
    property real maxHeight: root.hostHeight
    readonly property real inset: Math.round((QsLook.material ? 12 : 10) * root.s)
    property real rise: 1

    width: Math.max(1, Math.min(root.preferredWidth, root.hostWidth))
    implicitHeight: content.implicitHeight + topPadding + bottomPadding
    height: Math.max(1, Math.min(root.implicitHeight, root.maxHeight))
    padding: Math.min(Math.round((QsLook.material ? 8 : 6) * root.s), root.width / 4)
    modal: false
    focus: true
    transformOrigin: Item.BottomRight
    // a press on the session row, its gear included, is inside the parent, so the
    // gear closes the popup instead of the press closing it and the click reopening it
    closePolicy: QQC.Popup.CloseOnEscape | QQC.Popup.CloseOnPressOutside | QQC.Popup.CloseOnPressOutsideParent

    function openHub(section): void {
        root.close();
        root.requestClose();
        Bus.run(["hub", "open", section].join(" "));
    }

    SidebarPlugins {
        id: pluginHost
        active: root.active && SidebarState.elementVisible("pluginCards")
    }

    enter: Transition {
        ParallelAnimation {
            NumberAnimation {
                property: "opacity"
                from: 0
                to: 1
                duration: QsLook.dur(Math.round(Look.viewMs * 0.7))
                easing.type: Easing.OutCubic
            }
            NumberAnimation {
                property: "rise"
                from: 0
                to: 1
                duration: QsLook.dur(Math.round(Look.viewMs * 0.7))
                easing.type: Easing.BezierSpline
                easing.bezierCurve: QsLook.arrive
            }
            NumberAnimation {
                property: "scale"
                from: QsLook.material ? 0.94 : 1
                to: 1
                duration: QsLook.dur(Math.round(Look.viewMs * 0.7))
                easing.type: Easing.BezierSpline
                easing.bezierCurve: QsLook.arrive
            }
        }
    }
    exit: Transition {
        ParallelAnimation {
            NumberAnimation {
                property: "opacity"
                from: 1
                to: 0
                duration: QsLook.dur(110)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: QsLook.leave
            }
            NumberAnimation {
                property: "rise"
                from: 1
                to: 0.5
                duration: QsLook.dur(110)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: QsLook.leave
            }
        }
    }

    background: Item {
        transform: Translate { y: Math.round(8 * root.s) * (1 - root.rise) }

        RectangularShadow {
            anchors.fill: plane
            offset.y: Math.round(QsLook.shadowY * root.s)
            blur: Math.round(QsLook.shadowBlur * root.s)
            radius: QsLook.lithic ? 0 : Look.cornerAt(Look.cutMenu, plane.width, plane.height, root.s)
            color: QsLook.shadow
        }
        Shape {
            id: plane
            anchors.fill: parent
            s: root.s
            cutSize: Look.cutMenu
            fill: Look.popover
            strokeColor: Look.popoverStroke
            strokeWidth: Qt.colorEqual(Look.popoverStroke, "transparent") ? 0 : Math.max(1, Math.round(root.s))
        }
    }

    contentItem: QQC.ScrollView {
        id: viewport
        implicitHeight: content.implicitHeight
        contentWidth: availableWidth
        // clip only when it scrolls so the focus rings round the rows stay whole
        clip: content.implicitHeight > viewport.height + 0.5
        transform: Translate { y: Math.round(8 * root.s) * (1 - root.rise) }
        QQC.ScrollBar.horizontal.policy: QQC.ScrollBar.AlwaysOff
        QQC.ScrollBar.vertical.policy: QQC.ScrollBar.AsNeeded

        Column {
            id: content
            width: viewport.availableWidth

            GroupHead {
                width: content.width
                s: root.s
                inset: root.inset
                text: I18n.tr("Opens at")
            }
            Item {
                width: content.width
                height: positions.height

                PositionPicker {
                    id: positions
                    x: root.inset
                    s: root.s
                    current: SidebarState.position
                    onChose: key => SidebarState.setPosition(key)
                }
                Column {
                    anchors.left: positions.right
                    anchors.leftMargin: Math.round(14 * root.s)
                    anchors.right: parent.right
                    anchors.rightMargin: root.inset
                    anchors.verticalCenter: positions.verticalCenter
                    spacing: Math.round(3 * root.s)

                    Text {
                        width: parent.width
                        text: positions.currentName
                        color: Lithic.fg
                        font.family: QsLook.sans
                        font.pixelSize: Math.round(14 * root.s)
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                        renderType: Text.NativeRendering
                    }
                    Text {
                        width: parent.width
                        text: I18n.tr("Pick a spot, or use the arrow keys")
                        color: Lithic.fg3
                        font.family: QsLook.sans
                        font.pixelSize: Math.round(12 * root.s)
                        wrapMode: Text.WordWrap
                        renderType: Text.NativeRendering
                    }
                }
            }
            Item {
                width: content.width
                height: Math.round(34 * root.s)

                Text {
                    x: root.inset
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: Math.round(2 * root.s)
                    text: I18n.tr("Edge gap")
                    color: Lithic.fg
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(13 * root.s)
                    renderType: Text.NativeRendering
                }
                Text {
                    anchors.right: parent.right
                    anchors.rightMargin: root.inset
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: Math.round(2 * root.s)
                    text: I18n.tr("%1 px").arg(SidebarState.edgeGap)
                    color: Lithic.fg2
                    font.family: QsLook.figures
                    font.pixelSize: Math.round(12 * QsLook.figureScale * root.s)
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                }
            }
            GapSlider {
                x: root.inset
                width: content.width - root.inset * 2
                s: root.s
            }

            GroupBreak {
                width: content.width
                s: root.s
            }
            GroupHead {
                width: content.width
                s: root.s
                inset: root.inset
                text: I18n.tr("Motion")
            }
            SidebarSegments {
                x: root.inset
                width: content.width - root.inset * 2
                s: root.s
                options: ["quick", "standard", "calm"]
                labels: ({ "quick": I18n.tr("Quick"), "standard": I18n.tr("Standard"), "calm": I18n.tr("Calm") })
                current: SidebarState.speed
                accessibleName: I18n.tr("Motion")
                onChose: key => SidebarState.setSpeed(key)
            }
            Text {
                x: root.inset
                width: content.width - root.inset * 2
                topPadding: Math.round(7 * root.s)
                text: I18n.tr("How fast the panel opens, closes and reveals its blocks")
                color: Lithic.fg3
                font.family: QsLook.sans
                font.pixelSize: Math.round(12 * root.s)
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }

            Column {
                width: content.width
                visible: SidebarState.elementVisible("pluginCards") && root.pluginCards.length > 0

                GroupBreak {
                    width: content.width
                    s: root.s
                }
                GroupHead {
                    width: content.width
                    s: root.s
                    inset: root.inset
                    text: I18n.tr("Extensions")
                }
                SettingsRow {
                    width: content.width
                    s: root.s
                    inset: root.inset
                    glyph: "extension"
                    text: I18n.tr("Extension cards")
                    onClicked: {
                        root.close();
                        root.openExtensions();
                    }
                }
            }

            GroupBreak {
                width: content.width
                s: root.s
            }
            GroupHead {
                width: content.width
                s: root.s
                inset: root.inset
                text: I18n.tr("Ryoku Settings")
            }
            Column {
                width: content.width
                spacing: Math.round((QsLook.fluent ? 2 : 0) * root.s)

                Repeater {
                    model: [
                        { label: I18n.tr("Customize this panel"), glyph: "tune", section: "quick-settings" },
                        { label: I18n.tr("Displays"), glyph: "display_settings", section: "displays" },
                        { label: I18n.tr("Sounds"), glyph: "volume_up", section: "sounds" },
                        { label: I18n.tr("Wi-Fi and internet"), glyph: "wifi", section: "wifi" },
                        { label: I18n.tr("Power and battery"), glyph: "battery_full", section: "power" },
                        { label: I18n.tr("Keyboard shortcuts"), glyph: "keyboard", section: "shortcuts" }
                    ]
                    delegate: SettingsRow {
                        required property var modelData
                        width: content.width
                        s: root.s
                        inset: root.inset
                        glyph: modelData.glyph
                        text: modelData.label
                        onClicked: root.openHub(modelData.section)
                    }
                }
            }
        }
    }

    component GroupHead: Item {
        id: head

        required property real s
        required property real inset
        property alias text: label.text

        implicitHeight: Math.round((QsLook.lithic ? 26 : 30) * head.s)

        Micro {
            id: label
            x: head.inset
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Math.round(8 * head.s)
            width: Math.min(implicitWidth, head.width - head.inset * 2)
            s: head.s
            color: QsLook.material ? Lithic.sel : Lithic.fg3
        }
        Rectangle {
            visible: QsLook.lithic
            anchors.left: label.right
            anchors.leftMargin: Math.round(8 * head.s)
            anchors.right: parent.right
            anchors.rightMargin: head.inset
            anchors.verticalCenter: label.verticalCenter
            height: Math.max(1, Math.round(head.s))
            color: Lithic.vein
        }
    }

    component GroupBreak: Item {
        id: room

        required property real s

        implicitHeight: Math.round((QsLook.lithic ? 10 : 13) * room.s)

        Rectangle {
            visible: QsLook.adwaita || QsLook.fluent
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: Math.max(1, Math.round(room.s))
            color: QsLook.adwaita ? Look.divider : Look.rule
        }
    }

    component SettingsRow: QQC.AbstractButton {
        id: row

        required property real s
        required property real inset
        property string glyph: ""

        implicitHeight: Math.round((QsLook.material ? 44 : QsLook.lithic ? 34 : 36) * row.s)
        padding: 0
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        Accessible.role: Accessible.Button
        Accessible.name: row.text
        Keys.onReturnPressed: row.clicked()
        Keys.onEnterPressed: row.clicked()
        Keys.onUpPressed: {
            const previous = row.nextItemInFocusChain(false);
            if (previous)
                previous.forceActiveFocus(Qt.BacktabFocusReason);
        }
        Keys.onDownPressed: {
            const next = row.nextItemInFocusChain(true);
            if (next)
                next.forceActiveFocus(Qt.TabFocusReason);
        }

        background: CutRect {
            cut: Look.cutRow * row.s + (QsLook.lithic && row.down ? Lithic.pressCutGrow * row.s : 0)
            fill: row.down ? QsLook.press : row.hovered ? QsLook.hover : "transparent"
            Behavior on fill {
                ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
            }
            Behavior on cut {
                NumberAnimation { duration: QsLook.dur(LithicTokens.motion.press) }
            }
        }
        contentItem: Item {
            Item {
                anchors.fill: parent
                anchors.topMargin: QsLook.lithic && row.down ? Lithic.pressSink * row.s : 0
                anchors.bottomMargin: -anchors.topMargin

                Symbol {
                    id: lead
                    x: row.inset
                    anchors.verticalCenter: parent.verticalCenter
                    s: row.s
                    name: row.glyph
                    size: QsLook.material ? 20 : 18
                    ink: row.hovered || row.visualFocus ? Lithic.fg : Lithic.fg2
                    chosen: row.hovered
                    Behavior on ink {
                        ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                    }
                }
                Text {
                    anchors.left: lead.right
                    anchors.leftMargin: Math.round((QsLook.material ? 14 : 12) * row.s)
                    anchors.right: chevron.left
                    anchors.rightMargin: Math.round(8 * row.s)
                    anchors.verticalCenter: parent.verticalCenter
                    text: row.text
                    color: Lithic.fg
                    font.family: QsLook.sans
                    font.pixelSize: Math.round((QsLook.material ? 14 : 13) * row.s)
                    font.weight: QsLook.material ? Font.Medium : Font.Normal
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
                Symbol {
                    id: chevron
                    anchors.right: parent.right
                    anchors.rightMargin: row.inset - (row.hovered ? Math.round(3 * row.s) : 0)
                    anchors.verticalCenter: parent.verticalCenter
                    s: row.s
                    name: "chevron_right"
                    size: 18
                    ink: row.hovered ? Lithic.fg2 : Lithic.fg3
                    Behavior on anchors.rightMargin {
                        NumberAnimation { duration: QsLook.dur(160); easing.type: Easing.OutCubic }
                    }
                }
            }
        }

        FocusFrame {
            s: row.s
            shown: row.visualFocus
            cut: Look.cutRow * row.s
        }
        HoverHandler {
            cursorShape: Qt.PointingHandCursor
        }
    }

    component PositionPicker: QQC.Control {
        id: picker

        required property real s
        property string current: ""
        signal chose(string key)

        readonly property var spots: [
            { key: "top-left", name: I18n.tr("Top left") },
            { key: "top-center", name: I18n.tr("Top centre") },
            { key: "top-right", name: I18n.tr("Top right") },
            { key: "center", name: I18n.tr("Centre") }
        ]
        readonly property int index: Math.max(0, picker.spots.findIndex(spot => spot.key === picker.current))
        readonly property string currentName: picker.spots[picker.index].name
        readonly property real cut: LithicTokens.cut.mid * picker.s
        readonly property real margin: Math.round(6 * picker.s)
        readonly property real strip: Math.round(5 * picker.s)
        readonly property real workTop: picker.margin * 0.5 + picker.strip + Math.round(4 * picker.s)
        readonly property real spotWidth: Math.round(24 * picker.s)
        readonly property real spotHeight: Math.round(22 * picker.s)

        function spotX(i): real {
            return i === 0 ? picker.margin
                : i === 2 ? picker.width - picker.margin - picker.spotWidth
                : Math.round((picker.width - picker.spotWidth) / 2);
        }
        function spotY(i): real {
            return i === 3 ? Math.round(picker.workTop + (picker.height - picker.workTop - picker.spotHeight) / 2)
                : picker.workTop;
        }
        function move(key): void {
            let next = picker.index;
            if (key === Qt.Key_Left)
                next = picker.index === 3 ? 0 : Math.max(0, picker.index - 1);
            else if (key === Qt.Key_Right)
                next = picker.index === 3 ? 2 : Math.min(2, picker.index + 1);
            else if (key === Qt.Key_Down)
                next = 3;
            else if (key === Qt.Key_Up)
                next = picker.index === 3 ? 1 : picker.index;
            if (next !== picker.index)
                picker.chose(picker.spots[next].key);
        }

        implicitWidth: Math.round(150 * picker.s)
        implicitHeight: Math.round(picker.implicitWidth * 10 / 16)
        padding: 0
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        Accessible.role: Accessible.Grouping
        Accessible.name: I18n.tr("Opens at")
        Accessible.description: picker.currentName
        Keys.onPressed: event => {
            if (event.key === Qt.Key_Left || event.key === Qt.Key_Right || event.key === Qt.Key_Up || event.key === Qt.Key_Down) {
                picker.move(event.key);
                event.accepted = true;
            }
        }

        background: CutRect {
            cut: picker.cut
            fill: QsLook.lithic ? Lithic.field : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.06 : 0.05)
            strokeColor: QsLook.lithic ? Lithic.vein
                : QsLook.material ? "transparent"
                : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.1 : 0.12)
            strokeWidth: Math.max(1, Math.round(picker.s))

            Rectangle {
                x: picker.margin
                y: Math.round(picker.margin * 0.5)
                width: parent.width - picker.margin * 2
                height: picker.strip
                radius: QsLook.lithic ? 0 : height / 2
                color: Lithic.alpha(Lithic.fg, QsLook.dark ? 0.22 : 0.18)
            }
        }

        contentItem: Item {
            Repeater {
                model: picker.spots

                delegate: QQC.AbstractButton {
                    id: spot

                    required property var modelData
                    required property int index
                    readonly property bool chosen: spot.index === picker.index

                    x: picker.spotX(spot.index)
                    y: picker.spotY(spot.index)
                    width: picker.spotWidth
                    height: picker.spotHeight
                    padding: 0
                    hoverEnabled: true
                    focusPolicy: Qt.NoFocus
                    Accessible.role: Accessible.RadioButton
                    Accessible.name: spot.modelData.name
                    Accessible.checkable: true
                    Accessible.checked: spot.chosen
                    onClicked: picker.chose(spot.modelData.key)

                    background: CutRect {
                        cut: LithicTokens.cut.token * picker.s
                        fill: spot.down ? QsLook.press : spot.hovered ? Lithic.alpha(Lithic.sel, 0.18) : "transparent"
                        strokeColor: spot.hovered ? Lithic.alpha(Lithic.sel, 0.7) : Lithic.alpha(Lithic.fg3, 0.55)
                        strokeWidth: Math.max(1, Math.round(picker.s))
                        Behavior on fill {
                            ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                        }
                    }
                    contentItem: Item {}
                    HoverHandler {
                        cursorShape: Qt.PointingHandCursor
                    }
                }
            }

            CutRect {
                x: picker.spotX(picker.index)
                y: picker.spotY(picker.index)
                width: picker.spotWidth
                height: picker.spotHeight
                cut: LithicTokens.cut.token * picker.s
                fill: Lithic.sel
                Behavior on x {
                    NumberAnimation {
                        duration: QsLook.dur(QsLook.material ? 360 : 220)
                        easing.type: Easing.BezierSpline
                        easing.bezierCurve: QsLook.material ? Look.springSoft : Look.outCubic
                    }
                }
                Behavior on y {
                    NumberAnimation {
                        duration: QsLook.dur(QsLook.material ? 360 : 220)
                        easing.type: Easing.BezierSpline
                        easing.bezierCurve: QsLook.material ? Look.springSoft : Look.outCubic
                    }
                }

                Column {
                    x: Math.round(4 * picker.s)
                    y: Math.round(5 * picker.s)
                    spacing: Math.round(3 * picker.s)

                    Repeater {
                        model: 2
                        delegate: Rectangle {
                            required property int index
                            width: Math.round((index === 0 ? 14 : 10) * picker.s)
                            height: Math.max(1, Math.round(2 * picker.s))
                            radius: QsLook.lithic ? 0 : height / 2
                            color: Lithic.alpha(Lithic.onSel, 0.7)
                        }
                    }
                }
            }
        }

        FocusFrame {
            s: picker.s
            shown: picker.visualFocus
            cut: picker.cut
        }
    }

    component GapSlider: QQC.Slider {
        id: gap

        required property real s
        readonly property real lift: gap.pressed ? 1 : gap.hovered || gap.visualFocus ? 0.5 : 0
        property real liftShown: gap.lift
        readonly property real thin: Math.max(2, Math.round((QsLook.material ? 16 : QsLook.lithic ? 2 : 4) * gap.s))
        readonly property real knob: Math.round((QsLook.material ? 4 : QsLook.lithic ? 6 : 18) * gap.s)
        readonly property real at: gap.leftPadding + gap.visualPosition * (gap.availableWidth - gap.knob) + gap.knob / 2

        implicitHeight: Math.round((QsLook.material ? 30 : 26) * gap.s)
        padding: 0
        from: 0
        to: SidebarState.maxEdgeGap
        stepSize: 1
        snapMode: QQC.Slider.SnapAlways
        value: SidebarState.edgeGap
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        Accessible.name: I18n.tr("Edge gap")
        Accessible.description: I18n.tr("Distance between Quick settings and the nearest screen edge")
        onMoved: SidebarState.setEdgeGap(gap.value)
        Behavior on liftShown {
            NumberAnimation { duration: QsLook.dur(120); easing.type: Easing.OutCubic }
        }

        background: Item {
            Rectangle {
                x: QsLook.material ? Math.min(parent.width - height, gap.at + Math.round(6 * gap.s)) : 0
                anchors.verticalCenter: parent.verticalCenter
                width: QsLook.material ? Math.max(height, parent.width - x) : parent.width
                height: gap.thin
                radius: QsLook.lithic ? 0 : height / 2
                color: QsLook.lithic ? Lithic.vein
                    : QsLook.material ? Lithic.container
                    : QsLook.adwaita ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.16 : 0.13)
                    : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.45 : 0.4)
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(QsLook.material ? height : 0,
                    QsLook.material ? gap.at - Math.round(6 * gap.s) : gap.at)
                height: gap.thin
                radius: QsLook.lithic ? 0 : height / 2
                color: Lithic.sel
            }
        }

        handle: Item {
            x: gap.at - width / 2
            y: Math.round((gap.height - height) / 2)
            width: gap.knob
            height: QsLook.material ? Math.round(30 * gap.s) : QsLook.lithic ? Math.round(16 * gap.s) : gap.knob

            CutRect {
                visible: QsLook.lithic
                anchors.fill: parent
                cut: Math.round(2 * gap.s)
                fill: Lithic.sel
                scale: 1 + gap.liftShown * 0.12
            }
            Rectangle {
                visible: QsLook.material
                anchors.centerIn: parent
                width: Math.round((4 - gap.liftShown * 1.5) * gap.s)
                height: parent.height
                radius: width / 2
                color: Lithic.sel
            }
            Rectangle {
                visible: QsLook.adwaita || QsLook.fluent
                anchors.fill: parent
                radius: width / 2
                color: QsLook.adwaita && QsLook.dark ? Lithic.fg : Lithic.stone4
                border.width: Math.max(1, Math.round(gap.s))
                border.color: QsLook.fluent ? Lithic.vein : Lithic.alpha(Lithic.scrim, QsLook.dark ? 0.5 : 0.2)
                scale: QsLook.adwaita ? 1 + gap.liftShown * 0.1 : 1

                Rectangle {
                    visible: QsLook.fluent
                    anchors.centerIn: parent
                    width: Math.round((10 + 3 * gap.liftShown * (gap.pressed ? -0.6 : 1)) * gap.s)
                    height: width
                    radius: width / 2
                    color: Lithic.sel
                }
            }
        }

        FocusFrame {
            s: gap.s
            shown: gap.visualFocus
            cut: QsLook.lithic ? 0 : Look.corner(Look.cutRow, gap.width / gap.s, gap.height / gap.s) * gap.s
        }
        HoverHandler {
            cursorShape: Qt.PointingHandCursor
        }
        WheelHandler {
            onWheel: event => {
                SidebarState.setEdgeGap(SidebarState.edgeGap + (event.angleDelta.y >= 0 ? 1 : -1) * 2);
                event.accepted = true;
            }
        }
    }
}
