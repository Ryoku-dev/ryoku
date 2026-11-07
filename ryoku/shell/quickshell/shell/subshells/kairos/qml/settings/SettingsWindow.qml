pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons
import shell.barkit as Pill
import "../quicksettings/kit" as K
import "." as S

// Kairos' own settings, opened from the island gear. It belongs to the bar
// style: it is instantiated by the kairos Scene, so it exists only while kairos
// is the active bar, and only the gear opens it. Chrome follows the island --
// the same near-black fill, hairline rim, accent and mono readouts.
PanelWindow {
    id: root

    required property var targetScreen
    property bool open: false
    signal closeRequested()

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(root.ink.r, root.ink.g, root.ink.b, a) }

    readonly property var routes: [
        { id: "island", icon: "crop_16_9", label: I18n.tr("Island"),
          desc: I18n.tr("How the clock island looks and where it floats.") },
        { id: "clock", icon: "schedule", label: I18n.tr("Clock"),
          desc: I18n.tr("The time the island shows and the date wheel it opens over.") },
        { id: "music", icon: "music_note", label: I18n.tr("Music"),
          desc: I18n.tr("The now-playing bubble that grows beside the island.") },
        { id: "weather", icon: "thermostat", label: I18n.tr("Weather"),
          desc: I18n.tr("The temperature unit on the island's weather card.") }
    ]
    property string route: "island"
    function routeDef(id) {
        for (var i = 0; i < root.routes.length; i++)
            if (root.routes[i].id === id) return root.routes[i];
        return root.routes[0];
    }

    property real reveal: root.open ? 1 : 0
    visible: reveal > 0.001
    Behavior on reveal {
        enabled: !Motion.reduce
        NumberAnimation { duration: Motion.morph; easing.type: Motion.easeStandard }
    }
    onOpenChanged: if (!root.open) root.route = "island"

    S.IslandSettings { id: cfg }

    screen: root.targetScreen
    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-kairos-settings"
    WlrLayershell.keyboardFocus: root.open ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    MouseArea {
        anchors.fill: parent
        enabled: root.open
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        onPressed: root.closeRequested()
    }

    RectangularShadow {
        anchors.fill: card
        radius: card.radius
        blur: 44
        spread: 2
        offset: Qt.vector2d(0, 14)
        color: Qt.rgba(0, 0, 0, 0.62)
        opacity: root.reveal
    }

    Rectangle {
        id: card
        anchors.centerIn: parent
        width: Math.min(640, root.width - 48)
        height: Math.min(548, root.height - 96)
        radius: 30
        color: root.fill
        border.width: 1
        border.color: root.dim(0.16)
        clip: true

        opacity: root.reveal
        focus: root.open
        Keys.onPressed: function (e) {
            if (e.key === Qt.Escape) { root.closeRequested(); e.accepted = true; }
        }
        MouseArea { anchors.fill: parent; onClicked: {} }

        Rectangle {
            id: rail
            anchors { top: parent.top; bottom: parent.bottom; left: parent.left }
            width: 188
            color: root.dim(0.04)
            Rectangle {
                anchors { top: parent.top; bottom: parent.bottom; right: parent.right }
                width: 1
                color: root.dim(0.10)
            }

            Column {
                anchors { top: parent.top; left: parent.left; right: parent.right }
                anchors.topMargin: 22
                anchors.leftMargin: 14
                anchors.rightMargin: 14
                spacing: 4

                Column {
                    width: parent.width
                    leftPadding: 10
                    bottomPadding: 14
                    spacing: 1
                    Text {
                        text: I18n.tr("Kairos")
                        color: root.ink
                        font.family: Theme.fontPrimary
                        font.pixelSize: 17
                        font.weight: Font.DemiBold
                    }
                    Text {
                        text: I18n.tr("Island settings")
                        color: root.dim(0.55)
                        font.family: Theme.fontPrimary
                        font.pixelSize: 11
                    }
                }

                Repeater {
                    model: root.routes
                    delegate: RailEntry { }
                }
            }
        }

        Column {
            id: content
            anchors {
                top: parent.top; left: rail.right; right: parent.right
                topMargin: 26; leftMargin: 26; rightMargin: 26
            }
            spacing: 6

            Text {
                text: root.routeDef(root.route).label
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 20
                font.weight: Font.DemiBold
            }
            Text {
                width: parent.width
                text: root.routeDef(root.route).desc
                color: root.dim(0.58)
                font.family: Theme.fontPrimary
                font.pixelSize: 12
                wrapMode: Text.WordWrap
            }
        }

        Loader {
            id: page
            anchors {
                top: content.bottom; topMargin: 22
                left: rail.right; right: parent.right
                leftMargin: 26; rightMargin: 26
            }
            sourceComponent: root.route === "clock" ? pageClock
                : root.route === "music" ? pageMusic
                : root.route === "weather" ? pageWeather : pageIsland
        }

        Rectangle {
            id: reset
            anchors {
                left: rail.right; right: parent.right; bottom: parent.bottom
                leftMargin: 26; rightMargin: 26; bottomMargin: 22
            }
            height: 40
            radius: 12
            color: resetHover.hovered ? root.dim(0.08) : "transparent"
            border.width: 1
            border.color: root.dim(0.14)
            Text {
                anchors.centerIn: parent
                text: I18n.tr("Reset island settings")
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 12
            }
            HoverHandler { id: resetHover }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: cfg.reset()
            }
        }
    }

    // ---- pages ----

    Component {
        id: pageIsland
        Column {
            width: page.width
            spacing: 20

            SliderRow {
                width: parent.width
                title: I18n.tr("Island size")
                subtitle: I18n.tr("Scales the clock island, resting and expanded.")
                from: 80; to: 130
                value: Math.round(cfg.scale * 100)
                suffix: "%"
                onCommitted: (v) => cfg.set("scale", v / 100)
            }
            SliderRow {
                width: parent.width
                title: I18n.tr("Top offset")
                subtitle: I18n.tr("How far the island floats below the screen's top edge.")
                from: 0; to: 24
                value: Math.round(cfg.topGap)
                suffix: " px"
                onCommitted: (v) => cfg.set("topGap", v)
            }
        }
    }

    Component {
        id: pageClock
        Column {
            width: page.width
            spacing: 6

            ToggleRow {
                width: parent.width
                title: I18n.tr("12-hour clock")
                subtitle: I18n.tr("Read the time as 1-12 instead of 00-23.")
                value: cfg.clock12h
                onToggled: (v) => cfg.set("clock12h", v)
            }
            ToggleRow {
                width: parent.width
                title: I18n.tr("Show seconds")
                subtitle: I18n.tr("Tick the clock every second.")
                value: cfg.clockSeconds
                onToggled: (v) => cfg.set("clockSeconds", v)
            }
            ToggleRow {
                width: parent.width
                title: I18n.tr("Date wheel")
                subtitle: I18n.tr("Roll the days over the clock when it opens.")
                value: cfg.dateWheel
                onToggled: (v) => cfg.set("dateWheel", v)
            }
            ToggleRow {
                width: parent.width
                title: I18n.tr("App tray")
                subtitle: I18n.tr("A caret under the clock opens the system tray.")
                value: cfg.tray
                onToggled: (v) => cfg.set("tray", v)
            }
        }
    }

    Component {
        id: pageMusic
        Column {
            width: page.width
            spacing: 6

            ToggleRow {
                width: parent.width
                title: I18n.tr("Music island")
                subtitle: I18n.tr("Show the now-playing bubble beside the clock.")
                value: cfg.music
                onToggled: (v) => cfg.set("music", v)
            }
            SliderRow {
                width: parent.width
                title: I18n.tr("Peek size")
                subtitle: I18n.tr("How far the bubble opens on hover.")
                from: 70; to: 140
                value: Math.round(cfg.musicPeek)
                suffix: " px"
                onCommitted: (v) => cfg.set("musicPeek", v)
            }
        }
    }

    Component {
        id: pageWeather
        Column {
            id: wxPage
            width: page.width
            spacing: 14
            property bool searchMode: Config.weatherLocation.length > 0

            Text {
                width: parent.width
                text: I18n.tr("Temperature unit")
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 14
                font.weight: Font.DemiBold
            }
            Text {
                width: parent.width
                text: I18n.tr("How the island's weather card reads temperature.")
                color: root.dim(0.55)
                font.family: Theme.fontPrimary
                font.pixelSize: 11
                wrapMode: Text.WordWrap
            }
            Row {
                spacing: 8

                K.QsChip {
                    text: "\u00B0C"
                    active: Config.weatherUnit !== "fahrenheit"
                    onClicked: Weather.setUnit("celsius")
                }
                K.QsChip {
                    text: "\u00B0F"
                    active: Config.weatherUnit === "fahrenheit"
                    onClicked: Weather.setUnit("fahrenheit")
                }
            }

            ToggleRow {
                width: parent.width
                title: I18n.tr("Show location")
                subtitle: I18n.tr("Show the place above the temperature on the weather card.")
                value: cfg.showLocation
                onToggled: (v) => cfg.set("showLocation", v)
            }

            Text {
                width: parent.width
                text: I18n.tr("Location")
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 14
                font.weight: Font.DemiBold
            }
            Text {
                width: parent.width
                text: I18n.tr("Find the place by network address, or search for a city.")
                color: root.dim(0.55)
                font.family: Theme.fontPrimary
                font.pixelSize: 11
                wrapMode: Text.WordWrap
            }
            Row {
                spacing: 8

                K.QsChip {
                    text: I18n.tr("Automatic")
                    active: !wxPage.searchMode
                    onClicked: {
                        wxPage.searchMode = false;
                        if (Config.weatherLocation.length > 0)
                            cfg.setGlobal("weatherLocation", "");
                    }
                }
                K.QsChip {
                    text: I18n.tr("Search a place")
                    active: wxPage.searchMode
                    onClicked: {
                        wxPage.searchMode = true;
                        Qt.callLater(function () { placeField.forceActiveFocus(); });
                    }
                }
            }
            Rectangle {
                visible: wxPage.searchMode
                width: parent.width
                height: 40
                radius: 12
                color: root.dim(0.12)

                TextInput {
                    id: placeField
                    anchors {
                        left: parent.left; right: parent.right
                        leftMargin: 12; rightMargin: 12
                        verticalCenter: parent.verticalCenter
                    }
                    text: Config.weatherLocation
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 13
                    selectionColor: root.accent
                    selectedTextColor: root.fill
                    onAccepted: cfg.setGlobal("weatherLocation", text)
                }
            }
            K.QsAction {
                visible: wxPage.searchMode
                text: I18n.tr("Apply")
                onClicked: cfg.setGlobal("weatherLocation", placeField.text)
            }
        }
    }

    // ---- kit ----

    component ToggleRow: Item {
        id: tr

        property string title: ""
        property string subtitle: ""
        property bool value: false
        signal toggled(bool v)

        implicitHeight: tr.subtitle.length > 0 ? 60 : 44
        width: parent ? parent.width : 0

        Column {
            anchors {
                left: parent.left; right: sw.left; rightMargin: 18
                verticalCenter: parent.verticalCenter
            }
            spacing: 2
            Text {
                width: parent.width
                text: tr.title
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 14
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            Text {
                visible: tr.subtitle.length > 0
                width: parent.width
                text: tr.subtitle
                color: root.dim(0.55)
                font.family: Theme.fontPrimary
                font.pixelSize: 11
                wrapMode: Text.WordWrap
                maximumLineCount: 2
                elide: Text.ElideRight
            }
        }

        Rectangle {
            id: sw
            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
            width: 42
            height: 24
            radius: 12
            color: tr.value ? root.accent : root.dim(0.16)
            Behavior on color {
                enabled: !Motion.reduce
                ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
            }
            Rectangle {
                width: 18
                height: 18
                radius: 9
                y: (parent.height - height) / 2
                x: tr.value ? parent.width - width - 3 : 3
                color: tr.value ? root.fill : root.dim(0.85)
                Behavior on x {
                    enabled: !Motion.reduce
                    NumberAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
                }
            }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: tr.toggled(!tr.value)
            }
        }
    }

    component SliderRow: Item {
        id: sr

        property string title: ""
        property string subtitle: ""
        property real from: 0
        property real to: 100
        property real value: 0
        property string suffix: ""
        property real live: value
        property bool dragging: false
        signal committed(real v)

        implicitHeight: (sr.subtitle.length > 0 ? 36 : 20) + 26
        width: parent ? parent.width : 0

        readonly property real frac: Math.max(0, Math.min(1,
            (sr.live - sr.from) / Math.max(0.0001, sr.to - sr.from)))
        function valAt(mx) {
            return sr.from + Math.max(0, Math.min(1, mx / Math.max(1, track.width)))
                * (sr.to - sr.from);
        }
        onValueChanged: if (!sr.dragging) sr.live = sr.value
        Component.onCompleted: sr.live = sr.value

        Column {
            anchors { top: parent.top; left: parent.left; right: parent.right }
            spacing: 2
            Row {
                width: parent.width
                Text {
                    width: parent.width - amount.width
                    text: sr.title
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 14
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }
                Text {
                    id: amount
                    width: implicitWidth
                    text: Math.round(sr.live) + sr.suffix
                    color: root.accent
                    font.family: Theme.mono
                    font.pixelSize: 13
                    horizontalAlignment: Text.AlignRight
                }
            }
            Text {
                visible: sr.subtitle.length > 0
                width: parent.width
                text: sr.subtitle
                color: root.dim(0.55)
                font.family: Theme.fontPrimary
                font.pixelSize: 11
                wrapMode: Text.WordWrap
                maximumLineCount: 1
                elide: Text.ElideRight
            }
        }

        Item {
            id: track
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            height: 20

            Rectangle {
                anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter }
                height: 6
                radius: 3
                color: root.dim(0.16)
            }
            Rectangle {
                anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                width: track.width * sr.frac
                height: 6
                radius: 3
                color: root.accent
            }
            Rectangle {
                width: 15
                height: 15
                radius: 7.5
                color: root.accent
                border.width: 2
                border.color: root.fill
                anchors.verticalCenter: parent.verticalCenter
                x: Math.max(0, Math.min(track.width - width, track.width * sr.frac - width / 2))
            }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onPressed: (m) => { sr.dragging = true; sr.live = sr.valAt(m.x); }
                onPositionChanged: (m) => { if (sr.dragging) sr.live = sr.valAt(m.x); }
                onReleased: {
                    if (sr.dragging) {
                        sr.dragging = false;
                        sr.committed(Math.round(sr.live));
                    }
                }
                onCanceled: sr.dragging = false
            }
        }
    }

    component RailEntry: Item {
        id: re

        required property var modelData
        readonly property bool active: root.route === re.modelData.id

        width: parent ? parent.width : 0
        height: 44

        Rectangle {
            anchors.fill: parent
            radius: 12
            color: re.active ? Qt.rgba(root.accent.r, root.accent.g, root.accent.b, 0.16)
                : (reHover.hovered ? root.dim(0.06) : "transparent")
            Behavior on color {
                enabled: !Motion.reduce
                ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
            }
        }
        Row {
            anchors { left: parent.left; leftMargin: 12; verticalCenter: parent.verticalCenter }
            spacing: 12
            Pill.MaterialIcon {
                anchors.verticalCenter: parent.verticalCenter
                text: re.modelData.icon
                color: re.active ? root.accent : root.dim(0.7)
                font.pixelSize: 18
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: re.modelData.label
                color: re.active ? root.ink : root.dim(0.72)
                font.family: Theme.fontPrimary
                font.pixelSize: 13
                font.weight: re.active ? Font.DemiBold : Font.Normal
            }
        }
        HoverHandler { id: reHover }
        MouseArea {
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            onClicked: root.route = re.modelData.id
        }
    }
}
