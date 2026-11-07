pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Blobs
import Ryoku.Ui.Singletons
import shell.barkit as Pill
import "../kit" as K
import "pages" as P
import "." as S

Scope {
    id: root

    property var scene: null
    property var targetScreen: null
    property bool open: false
    property point origin: Qt.point(0, 0)
    property string page: "bar"
    property bool _keepAlive: false

    signal closeRequested()
    signal closed()


    function normalizedPage(value) {
        if (value === "general")
            return "bar";
        if (value === "wave")
            return "sound";
        const pages = ["bar", "layout", "modules", "sound", "about"];
        return pages.indexOf(value) >= 0 ? value : "bar";
    }

    onOpenChanged: if (open) root._keepAlive = true

    LazyLoader {
        id: surfaceLoader

        active: root.open || root._keepAlive

        PanelWindow {
            id: surface

            property string currentPage: "bar"
            property real morphProgress: 0
            property real introBase: 0
            property real introSidebar: 0
            property real introContent: 0
            property bool dragged: false
            property bool openingPending: false
            property real sheetX: 0
            property real sheetY: 0
            property real dragOffsetX: 0
            property real dragOffsetY: 0
            property real dragStartOffsetX: 0
            property real dragStartOffsetY: 0

            readonly property bool requestedOpen: root.open
            readonly property string requestedPage: root.normalizedPage(root.page)
            readonly property bool geometryReady: width > 160 && height > 140
            readonly property real dropSize: 26
            readonly property real sheetRadius: K.Style.sheetRadius
            readonly property real sheetW: Math.max(dropSize,
                Math.min(980, width - 160))
            readonly property real sheetH: Math.max(dropSize,
                Math.min(660, height - 140))
            readonly property real railW: 208
            readonly property real screenOriginX: Math.max(0, Math.min(width,
                root.origin.x - (root.targetScreen ? root.targetScreen.x : 0)))
            readonly property real screenOriginY: Math.max(0, Math.min(height,
                root.origin.y - (root.targetScreen ? root.targetScreen.y : 0)))
            readonly property var routes: [
                { id: "bar", label: "Bar", kana: "帯" },
                { id: "layout", label: "Layout", kana: "配置" },
                { id: "modules", label: "Modules", kana: "部品" },
                { id: "sound", label: "Sound", kana: "音" },
                { id: "about", label: "About", kana: "情報" }
            ]
            readonly property bool moduleDragActive: currentPage === "layout"
                && pageLoader.item && pageLoader.item.dragActive === true

            function clampUnit(value) {
                return Math.max(0, Math.min(1, value));
            }

            function smoothStep(value) {
                const amount = surface.clampUnit(value);
                return amount * amount * (3 - 2 * amount);
            }

            function arcDown(value) {
                const amount = surface.clampUnit(value);
                return amount * amount * (2 - amount);
            }

            function clampSheetX(value) {
                return Math.max(0, Math.min(surface.width - surface.sheetW, value));
            }

            function clampSheetY(value) {
                return Math.max(0, Math.min(surface.height - surface.sheetH, value));
            }

            function placeSheet() {
                if (!surface.geometryReady)
                    return;
                const centreX = Math.round((surface.width - surface.sheetW) / 2);
                const centreY = Math.round((surface.height - surface.sheetH) / 2);
                if (!surface.dragged) {
                    surface.sheetX = centreX;
                    surface.sheetY = centreY;
                    surface.dragOffsetX = 0;
                    surface.dragOffsetY = 0;
                    return;
                }
                surface.sheetX = surface.clampSheetX(centreX + surface.dragOffsetX);
                surface.sheetY = surface.clampSheetY(centreY + surface.dragOffsetY);
                surface.dragOffsetX = surface.sheetX - centreX;
                surface.dragOffsetY = surface.sheetY - centreY;
            }

            function startOpenMorph() {
                if (!surface.requestedOpen || !surface.geometryReady)
                    return;
                surface.openingPending = false;
                surface.placeSheet();
                openMorph.restart();
                introDelay.restart();
            }

            function geometryChanged() {
                surface.placeSheet();
                if (surface.openingPending)
                    surface.startOpenMorph();
            }

            function showSheet() {
                root._keepAlive = true;
                closeSequence.stop();
                openMorph.stop();
                introDelay.stop();
                startupSequence.stop();
                surface.currentPage = surface.requestedPage;
                surface.dragged = false;
                surface.dragOffsetX = 0;
                surface.dragOffsetY = 0;
                surface.openingPending = true;
                surface.introBase = 0;
                surface.introSidebar = 0;
                surface.introContent = 0;
                surface.placeSheet();
                surface.startOpenMorph();
            }

            function hideSheet() {
                S.Prefs.flush();
                surface.openingPending = false;
                openMorph.stop();
                introDelay.stop();
                startupSequence.stop();
                closeSequence.restart();
            }

            function finishClose() {
                if (root.open) {
                    root._keepAlive = true;
                    surface.showSheet();
                } else {
                    root.closed();
                    root._keepAlive = false;
                }
            }


            screen: root.targetScreen
            visible: true
            color: "transparent"
            anchors { top: true; bottom: true; left: true; right: true }
            exclusionMode: ExclusionMode.Ignore
            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.namespace: "ryoku-orpheus-settings"
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive

            onRequestedOpenChanged: requestedOpen ? showSheet() : hideSheet()
            onRequestedPageChanged: currentPage = requestedPage
            onCurrentPageChanged: Qt.callLater(function() {
                if (pageScroll)
                    pageScroll.contentY = 0;
            })
            onWidthChanged: Qt.callLater(geometryChanged)
            onHeightChanged: Qt.callLater(geometryChanged)
            onSheetWChanged: Qt.callLater(geometryChanged)
            onSheetHChanged: Qt.callLater(geometryChanged)
            Component.onCompleted: Qt.callLater(function() {
                surface.currentPage = surface.requestedPage;
                if (surface.requestedOpen)
                    surface.showSheet();
                else
                    surface.geometryChanged();
            })

            FocusScope {
                anchors.fill: parent
                focus: true
            }

            Shortcut {
                sequences: [StandardKey.Cancel]
                enabled: surface.requestedOpen && !surface.moduleDragActive
                onActivated: root.closeRequested()
            }

            Rectangle {
                anchors.fill: parent
                color: Qt.rgba(0, 0, 0, 0.4)
                opacity: Math.min(1, surface.morphProgress * 1.5)

                MouseArea {
                    anchors.fill: parent
                    enabled: surface.requestedOpen && !surface.moduleDragActive
                    acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                    onPressed: root.closeRequested()
                }
            }

            BlobGroup {
                id: liquid

                color: K.Style.base
                smoothing: 12
                borderWidth: 1
                borderColor: K.Style.lineSoft
                shadowStrength: 0
                shadowSize: 0
            }

            BlobRect {
                z: 1
                group: liquid
                readonly property real travel: Math.min(1, surface.morphProgress / 0.6)
                readonly property real xProgress: surface.smoothStep(travel)
                readonly property real yProgress: surface.arcDown(travel)
                readonly property real swell: surface.smoothStep(surface.morphProgress / 0.45)
                readonly property real growthProgress: surface.smoothStep(
                    (surface.morphProgress - 0.45) / 0.55)
                readonly property real cornerProgress: surface.smoothStep(
                    (surface.morphProgress - 0.65) / 0.35)
                readonly property real dropW: surface.dropSize + 30 * swell
                readonly property real roundRadius: Math.min(width, height) / 2
                width: dropW + (surface.sheetW - dropW) * growthProgress
                height: dropW + (surface.sheetH - dropW) * growthProgress
                x: surface.screenOriginX
                    + (surface.sheetX + surface.sheetW / 2 - surface.screenOriginX)
                        * xProgress - width / 2
                y: surface.screenOriginY
                    + (surface.sheetY + surface.sheetH / 2 - surface.screenOriginY)
                        * yProgress - height / 2
                radius: roundRadius
                    + (surface.sheetRadius - roundRadius) * cornerProgress
                stiffness: 180
                damping: 14
                deformScale: 0.0016 * Math.pow(Math.min(1,
                    (surface.dropSize + 30) / Math.max(1, width)), 2)
                sinks: false
            }

            Item {
                id: sheet

                z: 2
                x: surface.sheetX
                y: surface.sheetY
                width: surface.sheetW
                height: surface.sheetH
                clip: true
                visible: surface.introBase > 0
                enabled: surface.requestedOpen
                opacity: surface.introBase
                scale: 0.95 + 0.05 * surface.introBase

                MouseArea {
                    anchors.fill: parent
                    acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                    onPressed: mouse => mouse.accepted = true
                }

                Item {
                    id: rail

                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.left: parent.left
                    width: surface.railW
                    opacity: surface.introSidebar
                    transform: Translate { x: -24 * (1 - surface.introSidebar) }

                    Rectangle {
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        anchors.right: parent.right
                        width: 1
                        color: K.Style.lineSoft
                    }

                    Rectangle {
                        id: masthead

                        anchors.top: parent.top
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.topMargin: K.Style.s4
                        anchors.leftMargin: K.Style.s4
                        anchors.rightMargin: K.Style.s4
                        height: 72
                        radius: K.Style.radius
                        color: "transparent"
                        border.width: 1
                        border.color: K.Style.lineSoft

                        Pill.BrandMark {
                            id: mastheadSeal
                            anchors.left: parent.left
                            anchors.leftMargin: K.Style.s3
                            anchors.verticalCenter: parent.verticalCenter
                            size: 24
                            color: K.Style.accent
                            weight: Font.Black
                        }

                        Column {
                            anchors.left: mastheadSeal.right
                            anchors.leftMargin: K.Style.s3
                            anchors.right: parent.right
                            anchors.rightMargin: K.Style.s3
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: K.Style.s1

                            K.Label {
                                width: parent.width
                                text: "ORPHEUS"
                                color: K.Style.ink
                                font.family: K.Style.font
                                font.pixelSize: 13
                                font.weight: Font.Medium
                                font.capitalization: Font.AllUppercase
                                font.letterSpacing: K.Style.track
                            }

                            K.Label {
                                width: parent.width
                                text: "//SETTINGS_"
                                role: "tag"
                                tone: "faint"
                            }
                        }
                    }

                    Column {
                        id: navColumn

                        anchors.top: masthead.bottom
                        anchors.topMargin: K.Style.s4
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.leftMargin: K.Style.s3
                        anchors.rightMargin: K.Style.s3
                        spacing: K.Style.s1

                        Repeater {
                            model: surface.routes

                            delegate: Item {
                                id: navItem

                                required property var modelData
                                readonly property bool active:
                                    surface.currentPage === modelData.id

                                width: navColumn.width
                                height: 38
                                scale: navPress.pressScale

                                Rectangle {
                                    anchors.fill: parent
                                    radius: K.Style.radius
                                    color: navItem.active ? K.Style.bone
                                        : navPress.pressed ? K.Style.tint16
                                        : navPress.hovered ? K.Style.tint5
                                        : "transparent"

                                    Behavior on color {
                                        enabled: !K.Style.still
                                        ColorAnimation { duration: K.Style.snap }
                                    }
                                }

                                K.Label {
                                    id: navLead
                                    anchors.left: parent.left
                                    anchors.leftMargin: K.Style.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 18
                                    text: "//"
                                    role: "micro"
                                    tone: navItem.active ? "onBone" : "faint"
                                    opacity: navItem.active ? 1 : 0
                                }

                                K.Label {
                                    anchors.left: navLead.right
                                    anchors.leftMargin: K.Style.s1
                                    anchors.right: navGloss.left
                                    anchors.rightMargin: K.Style.s2
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: navItem.modelData.label
                                    color: navItem.active ? K.Style.inkOnBone : K.Style.ink
                                    font.family: K.Style.font
                                    font.pixelSize: 14
                                    font.weight: navItem.active ? Font.Medium : Font.Normal
                                }

                                K.Label {
                                    id: navGloss
                                    anchors.right: parent.right
                                    anchors.rightMargin: K.Style.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: UiPreferences.text(navItem.modelData.kana, "")
                                    color: navItem.active ? K.Style.inkOnBone : K.Style.inkFaint
                                    font.family: K.Style.jp
                                    font.pixelSize: 12
                                }

                                K.Press {
                                    id: navPress
                                    enabled: !surface.moduleDragActive
                                    tint: false
                                    radius: K.Style.radius
                                    onClicked: surface.currentPage = navItem.modelData.id
                                }
                            }
                        }
                    }

                    Row {
                        id: saveStatus
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        anchors.leftMargin: K.Style.s4
                        anchors.rightMargin: K.Style.s4
                        anchors.bottomMargin: K.Style.s4
                        spacing: K.Style.s2

                        Rectangle {
                            anchors.verticalCenter: parent.verticalCenter
                            width: 6
                            height: 6
                            radius: 3
                            color: S.Prefs.saveState === "failed" ? K.Style.alert
                                : S.Prefs.saveState === "saving" ? K.Style.inkFaint : K.Style.ink
                        }

                        K.Label {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - 6 - parent.spacing
                            text: S.Prefs.saveState === "failed" ? "Not saved"
                                : S.Prefs.saveState === "saving" ? "Saving" : "Saved"
                            role: "micro"
                            tone: S.Prefs.saveState === "failed" ? "alert" : "muted"
                        }

                        HoverHandler { id: saveHover }
                    }

                    K.Hint {
                        target: saveStatus
                        text: S.Prefs.saveError
                        shown: S.Prefs.saveError.length > 0 && saveHover.hovered
                    }
                }

                Item {
                    id: pageRegion

                    anchors.top: parent.top
                    anchors.topMargin: K.Style.s6
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: K.Style.s5
                    anchors.left: rail.right
                    anchors.leftMargin: K.Style.s6
                    anchors.right: parent.right
                    anchors.rightMargin: K.Style.s6
                    opacity: surface.introContent
                    transform: Translate {
                        y: K.Style.s3 * (1 - surface.introContent)
                    }

                    K.Scroller {
                        id: pageScroll

                        anchors.fill: parent
                        interactive: !surface.moduleDragActive
                            && contentHeight > height + 0.5

                        Loader {
                            id: pageLoader

                            width: Math.max(0, pageScroll.width - K.Style.s2)
                            height: item ? item.implicitHeight : 0
                            sourceComponent: surface.currentPage === "layout" ? layoutPage
                                : surface.currentPage === "modules" ? modulesPage
                                : surface.currentPage === "sound" ? soundPage
                                : surface.currentPage === "about" ? aboutPage
                                : barPage
                        }
                    }
                }

                Item {
                    z: 20
                    anchors.top: parent.top
                    anchors.left: parent.left
                    anchors.right: parent.right
                    height: K.Style.s4
                    enabled: !surface.moduleDragActive

                    HoverHandler {
                        cursorShape: sheetDrag.active
                            ? Qt.ClosedHandCursor : Qt.OpenHandCursor
                    }

                    DragHandler {
                        id: sheetDrag

                        target: null
                        enabled: !surface.moduleDragActive
                        acceptedButtons: Qt.LeftButton
                        onActiveChanged: if (active) {
                            surface.dragged = true;
                            surface.dragStartOffsetX = surface.dragOffsetX;
                            surface.dragStartOffsetY = surface.dragOffsetY;
                        }
                        onTranslationChanged: if (active) {
                            surface.dragOffsetX = surface.dragStartOffsetX + translation.x;
                            surface.dragOffsetY = surface.dragStartOffsetY + translation.y;
                            surface.placeSheet();
                        }
                    }
                }
            }

            Component {
                id: barPage

                P.BarPage {
                    width: pageLoader.width
                    scene: root.scene
                    shown: surface.introContent > 0.1
                }
            }

            Component {
                id: layoutPage

                P.LayoutPage {
                    width: pageLoader.width
                    scene: root.scene
                    shown: surface.introContent > 0.1
                    dragHost: sheet
                }
            }

            Component {
                id: modulesPage

                P.ModulesPage {
                    width: pageLoader.width
                    scene: root.scene
                    shown: surface.introContent > 0.1
                }
            }

            Component {
                id: soundPage

                P.SoundPage {
                    width: pageLoader.width
                    scene: root.scene
                    shown: surface.introContent > 0.1
                }
            }

            Component {
                id: aboutPage

                P.AboutPage {
                    width: pageLoader.width
                    scene: root.scene
                    shown: surface.introContent > 0.1
                }
            }

            NumberAnimation {
                id: openMorph

                target: surface
                property: "morphProgress"
                to: 1
                duration: K.Style.ms(520)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: K.Style.spatialCurve
            }

            SequentialAnimation {
                id: introDelay

                PauseAnimation { duration: K.Style.ms(364) }
                ScriptAction {
                    script: {
                        if (surface.requestedOpen)
                            startupSequence.restart();
                    }
                }
            }

            ParallelAnimation {
                id: startupSequence

                NumberAnimation {
                    target: surface
                    property: "introBase"
                    to: 1
                    duration: K.Style.ms(220)
                    easing.type: Easing.OutExpo
                }

                SequentialAnimation {
                    PauseAnimation { duration: K.Style.ms(28) }
                    NumberAnimation {
                        target: surface
                        property: "introSidebar"
                        to: 1
                        duration: K.Style.ms(240)
                        easing.type: Easing.OutCubic
                    }
                }

                SequentialAnimation {
                    PauseAnimation { duration: K.Style.ms(84) }
                    NumberAnimation {
                        target: surface
                        property: "introContent"
                        to: 1
                        duration: K.Style.ms(360)
                        easing.type: Easing.OutCubic
                    }
                }
            }

            SequentialAnimation {
                id: closeSequence

                ParallelAnimation {
                    NumberAnimation {
                        target: surface
                        property: "introContent"
                        to: 0
                        duration: K.Style.ms(120)
                        easing.type: Easing.InExpo
                    }
                    NumberAnimation {
                        target: surface
                        property: "introSidebar"
                        to: 0
                        duration: K.Style.ms(120)
                        easing.type: Easing.InExpo
                    }
                    NumberAnimation {
                        target: surface
                        property: "introBase"
                        to: 0
                        duration: K.Style.ms(120)
                        easing.type: Easing.InQuart
                    }
                }
                NumberAnimation {
                    target: surface
                    property: "morphProgress"
                    to: 0
                    duration: K.Style.ms(320)
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: K.Style.spatialCurve
                }
                ScriptAction { script: surface.finishClose() }
            }
        }
    }
}
