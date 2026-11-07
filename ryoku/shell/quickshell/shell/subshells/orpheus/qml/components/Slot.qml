pragma ComponentBehavior: Bound

import QtQuick
import "../kit" as K

Item {
    id: slot

    required property var scene
    required property string lane
    required property string moduleId
    required property string file

    property bool failed: false
    property real appear: present ? 1 : 0

    readonly property bool ready: moduleLoader.status === Loader.Ready
        && moduleLoader.item !== null && !failed
    readonly property var moduleItem: ready ? moduleLoader.item : null
    readonly property bool present: ready && moduleLoader.item.present !== false
    readonly property real naturalWidth: ready
        ? Math.max(0, Number(moduleLoader.item.implicitWidth) || 0) : 0
    readonly property real targetWidth: present ? naturalWidth : 0
    readonly property real reveal: Math.max(0, Math.min(1, appear))

    width: targetWidth
    height: K.Style.barH
    clip: true
    visible: width > 0.001 || targetWidth > 0.001

    function load() {
        slot.failed = false;
        if (!slot.file || !slot.scene || !slot.lane) {
            moduleLoader.source = "";
            return;
        }
        moduleLoader.setSource(Qt.resolvedUrl("../modules/" + slot.file), {
            scene: slot.scene,
            lane: slot.lane
        });
    }

    Behavior on appear {
        enabled: !K.Style.still
        NumberAnimation {
            duration: K.Style.effects
            easing.type: Easing.BezierSpline
            easing.bezierCurve: K.Style.effectsCurve
        }
    }
    onFileChanged: load()
    Component.onCompleted: load()

    Behavior on width {
        enabled: !K.Style.still
        SpringAnimation {
            spring: 3.6
            damping: 0.42
            mass: 1
            epsilon: 0.5
        }
    }

    // Content holds still on screen while the slot springs: centre drops open
    // both ways and right drops open leftward.
    Loader {
        id: moduleLoader
        x: slot.lane === "center" ? Math.round((slot.width - slot.naturalWidth) / 2)
            : slot.lane === "right" ? Math.round(slot.width - slot.naturalWidth) : 0
        y: Math.round((slot.height - height) / 2)
        width: slot.naturalWidth
        height: slot.height
        opacity: slot.reveal
        visible: slot.ready

        onStatusChanged: if (status === Loader.Error) slot.failed = true
    }
}
