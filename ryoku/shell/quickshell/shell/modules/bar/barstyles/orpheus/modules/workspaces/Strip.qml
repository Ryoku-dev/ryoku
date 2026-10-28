pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Blobs
import shell.services
import Ryoku.Ui.Singletons
import "../../kit" as K

Item {
    id: root

    property string style: "dots"
    property var model: []
    property bool interactive: true
    property real unit: K.Style.barH
    property bool square: false

    signal activated(var key)
    signal scrolled(int steps)
    signal overviewRequested()

    readonly property real previewScale: unit / Math.max(1, K.Style.barH)
    readonly property real u: K.Style.scale * previewScale
    readonly property real pitch: 24 * u
    readonly property real smoothing: square ? 3 : 8

    // Square marks keep a small corner; liquid marks are full pills.
    function cornerFor(height) {
        return square ? Math.min(3 * u, height / 2) : height / 2;
    }
    readonly property color occupiedColor: Theme.blend(K.Style.tint16, K.Style.base)
    readonly property string requestedMode: ["dots", "numbers", "kanji", "greek", "dashes", "icons"].indexOf(style) >= 0
        ? style : "dots"
    readonly property string mode: requestedMode === "kanji" && !UiPreferences.showJapaneseGloss ? "numbers" : requestedMode
    readonly property int activeIndex: {
        const rows = model || [];
        for (let index = 0; index < rows.length; ++index) {
            if (rows[index].active === true)
                return index;
        }
        return -1;
    }
    readonly property real activeBaseWidth: mode === "dashes" ? 22 * u : 26 * u
    readonly property real activeHeight: mode === "dashes" ? 3 * u : 18 * u
    readonly property real activeX: Math.min(leftEdge, rightEdge) * pitch
        + (pitch - activeBaseWidth) / 2
    readonly property real activeWidth: Math.abs(rightEdge - leftEdge) * pitch + activeBaseWidth

    property real leftEdge: 0
    property real rightEdge: 0
    property int settledIndex: -1

    implicitWidth: (model ? model.length : 0) * pitch
    implicitHeight: unit
    width: implicitWidth
    height: implicitHeight

    function numberFor(data, fallback) {
        const parsed = Number(data && data.index);
        return isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : fallback + 1;
    }

    function numberText(data, fallback) {
        const label = String((data && data.label) || "").trim();
        return /^\d+$/.test(label) ? label : String(numberFor(data, fallback));
    }

    function kanjiText(data, fallback) {
        const n = numberFor(data, fallback);
        const digit = ["", "一", "二", "三", "四", "五", "六", "七", "八", "九"];
        if (n < 10)
            return digit[n];
        if (n === 10)
            return "十";
        if (n < 20)
            return "十" + digit[n - 10];
        if (n < 100) {
            const tens = Math.floor(n / 10);
            const ones = n % 10;
            return digit[tens] + "十" + digit[ones];
        }
        return "百";
    }

    function greekText(data, fallback) {
        const letters = ["α", "β", "γ", "δ", "ε", "ζ", "η", "θ", "ι", "κ", "λ", "μ",
            "ν", "ξ", "ο", "π", "ρ", "σ", "τ", "υ", "φ", "χ", "ψ", "ω"];
        return letters[(numberFor(data, fallback) - 1) % letters.length];
    }

    function windowCount(data) {
        const reported = Number(data && data.windowCount);
        if (isFinite(reported) && reported >= 0)
            return Math.floor(reported);
        return data && data.apps ? data.apps.length : 0;
    }

    function hintFor(data, fallback) {
        const count = windowCount(data);
        return "Workspace " + numberText(data, fallback) + ", " + count
            + (count === 1 ? " window" : " windows");
    }

    function moveIndicator(next) {
        if (next < 0) {
            settledIndex = -1;
            return;
        }
        if (settledIndex < 0 || K.Style.still) {
            leftAnimation.stop();
            rightAnimation.stop();
            leftEdge = next;
            rightEdge = next;
            settledIndex = next;
            return;
        }

        const movingRight = next > settledIndex;
        settledIndex = next;
        leftAnimation.stop();
        rightAnimation.stop();
        leftAnimation.from = leftEdge;
        leftAnimation.to = next;
        leftAnimation.duration = K.Style.ms(movingRight ? 380 : 200);
        rightAnimation.from = rightEdge;
        rightAnimation.to = next;
        rightAnimation.duration = K.Style.ms(movingRight ? 200 : 380);
        leftAnimation.restart();
        rightAnimation.restart();
    }

    onActiveIndexChanged: moveIndicator(activeIndex)
    Component.onCompleted: moveIndicator(activeIndex)

    NumberAnimation {
        id: leftAnimation
        target: root
        property: "leftEdge"
        easing.bezierCurve: K.Style.spatialFastCurve
    }

    NumberAnimation {
        id: rightAnimation
        target: root
        property: "rightEdge"
        easing.bezierCurve: K.Style.spatialFastCurve
    }


    BlobGroup {
        id: occupiedGroup
        color: root.occupiedColor
        smoothing: root.smoothing
        borderWidth: 0
        shadowStrength: 0
        shadowSize: 0
    }

    Repeater {
        model: root.model || []

        delegate: BlobRect {
            id: occupiedDrop
            required property var modelData
            required property int index

            readonly property bool occupied: modelData.occupied === true
                || root.windowCount(modelData) > 0

            group: occupiedGroup
            x: index * root.pitch + (root.pitch - implicitWidth) / 2
            y: (root.height - implicitHeight) / 2
            implicitWidth: occupied && root.mode !== "dashes" ? 16 * root.u : 0
            implicitHeight: 16 * root.u
            opacity: implicitWidth > 0 ? 1 : 0
            topLeftRadius: root.cornerFor(implicitHeight)
            topRightRadius: root.cornerFor(implicitHeight)
            bottomLeftRadius: root.cornerFor(implicitHeight)
            bottomRightRadius: root.cornerFor(implicitHeight)
            deformScale: 0

            Behavior on implicitWidth {
                enabled: !K.Style.still
                NumberAnimation {
                    duration: K.Style.spatialFast
                    easing.bezierCurve: K.Style.spatialFastCurve
                }
            }
        }
    }

    BlobGroup {
        id: urgentGroup
        color: K.Style.alert
        smoothing: root.smoothing
        borderWidth: 0
        shadowStrength: 0
        shadowSize: 0
    }

    Repeater {
        model: root.model || []

        delegate: BlobRect {
            required property var modelData
            required property int index

            readonly property bool urgent: modelData.urgent === true
                && modelData.active !== true

            group: urgentGroup
            x: index * root.pitch + (root.pitch - implicitWidth) / 2
            y: (root.height - implicitHeight) / 2
            implicitWidth: urgent && root.mode !== "dashes" ? 16 * root.u : 0
            implicitHeight: 16 * root.u
            opacity: implicitWidth > 0 ? 1 : 0
            topLeftRadius: root.cornerFor(implicitHeight)
            topRightRadius: root.cornerFor(implicitHeight)
            bottomLeftRadius: root.cornerFor(implicitHeight)
            bottomRightRadius: root.cornerFor(implicitHeight)
            deformScale: 0

            Behavior on implicitWidth {
                enabled: !K.Style.still
                NumberAnimation {
                    duration: K.Style.spatialFast
                    easing.bezierCurve: K.Style.spatialFastCurve
                }
            }
        }
    }

    BlobGroup {
        id: activeGroup
        color: K.Style.bone
        smoothing: root.smoothing
        borderWidth: 0
        shadowStrength: 0
        shadowSize: 0
    }

    BlobRect {
        id: activeDrop
        z: 1
        visible: root.activeIndex >= 0
        group: activeGroup
        x: root.activeX
        y: (root.height - implicitHeight) / 2
        implicitWidth: root.activeWidth
        implicitHeight: root.activeHeight
        opacity: 1
        topLeftRadius: root.cornerFor(implicitHeight)
        topRightRadius: root.cornerFor(implicitHeight)
        bottomLeftRadius: root.cornerFor(implicitHeight)
        bottomRightRadius: root.cornerFor(implicitHeight)
        deformScale: 0
    }

    Row {
        id: slots
        z: 2
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        spacing: 0

        Repeater {
            model: root.model || []

            delegate: Item {
                id: slot
                required property var modelData
                required property int index

                readonly property var apps: modelData.apps || []
                readonly property bool occupied: modelData.occupied === true
                    || root.windowCount(modelData) > 0
                readonly property bool active: modelData.active === true

                width: root.pitch
                height: root.unit

                Rectangle {
                    anchors.centerIn: parent
                    visible: root.mode === "dots" && !slot.occupied && !slot.active
                    width: 5 * root.u
                    height: width
                    radius: width / 2
                    color: K.Style.inkFaint
                }

                Rectangle {
                    anchors.centerIn: parent
                    visible: root.mode === "dashes" && !slot.active
                    width: (slot.active ? 22 : slot.occupied ? 12 : 8) * root.u
                    height: 3 * root.u
                    radius: height / 2
                    color: slot.modelData.urgent === true ? K.Style.alert
                        : slot.occupied ? root.occupiedColor : K.Style.inkFaint
                }

                K.Label {
                    anchors.centerIn: parent
                    visible: root.mode === "numbers" || root.mode === "kanji"
                        || root.mode === "greek"
                    text: root.mode === "numbers" ? root.numberText(slot.modelData, slot.index)
                        : root.mode === "kanji" ? root.kanjiText(slot.modelData, slot.index)
                        : root.greekText(slot.modelData, slot.index)
                    role: "label"
                    tone: slot.active ? "onBone"
                        : slot.modelData.urgent === true ? "alert"
                        : slot.occupied ? "ink" : "faint"
                    numeric: root.mode === "numbers"
                    font.family: root.mode === "kanji" ? Tokens.jp : K.Style.font
                }

                Item {
                    anchors.centerIn: parent
                    visible: root.mode === "icons"
                    width: Math.min(root.pitch, iconRow.implicitWidth)
                    height: 14 * root.u

                    Row {
                        id: iconRow
                        anchors.centerIn: parent
                        spacing: slot.apps.length >= 3 ? -9 * root.u
                            : slot.apps.length === 2 ? -4 * root.u : 0

                        Rectangle {
                            visible: slot.apps.length === 0
                            anchors.verticalCenter: parent.verticalCenter
                            width: 5 * root.u
                            height: width
                            radius: width / 2
                            color: slot.active ? K.Style.inkOnBone
                                : slot.modelData.urgent === true ? K.Style.alert
                                : slot.occupied ? K.Style.ink : K.Style.inkFaint
                        }

                        Repeater {
                            model: slot.apps.slice(0, 3)

                            delegate: Item {
                                id: appIcon
                                required property var modelData

                                readonly property url iconSource: Icons.path(String(modelData || ""), true)

                                width: 14 * root.u
                                height: width

                                Image {
                                    anchors.fill: parent
                                    visible: appIcon.iconSource.toString().length > 0
                                        && status !== Image.Error
                                    source: appIcon.iconSource
                                    sourceSize: Qt.size(Math.max(1, Math.round(width * Screen.devicePixelRatio * 2)),
                                        Math.max(1, Math.round(height * Screen.devicePixelRatio * 2)))
                                    fillMode: Image.PreserveAspectFit
                                    asynchronous: true
                                    smooth: true
                                    mipmap: true
                                }

                                K.Glyph {
                                    anchors.centerIn: parent
                                    visible: appIcon.iconSource.toString().length === 0
                                    name: "web_asset"
                                    size: 12 * root.u
                                    tone: slot.active ? "onBone"
                                        : slot.modelData.urgent === true ? "alert"
                                        : slot.occupied ? "ink" : "faint"
                                }
                            }
                        }
                    }
                }

                K.Press {
                    id: press
                    enabled: root.interactive
                    radius: K.Style.radius
                    onDown: mouse => {
                        if (mouse.button === Qt.LeftButton)
                            root.activated(slot.modelData.key);
                    }
                    onRightClicked: root.overviewRequested()
                    wheelSteps: true
                    onWheel: steps => root.scrolled(steps)
                }

                K.Hint {
                    target: slot
                    text: root.hintFor(slot.modelData, slot.index)
                    shown: root.interactive && press.hovered
                }
            }
        }
    }
}
