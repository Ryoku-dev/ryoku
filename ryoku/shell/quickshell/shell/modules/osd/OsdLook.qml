pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "designs"

// Ryoku's own pop-up in the desktop's design language (theme.design), switched
// live: each look owns its shape, its place on the screen and its motion. The
// window reads where to stand (placement, gap from that edge) and how far the
// look is still revealed, so it stays mapped until the exit has finished.
// Each sits where its own platform puts it, against the system's chrome.
Loader {
    id: root

    property string kind: "volume"
    property real u: 1
    property bool shown: false
    property real value: 0
    property real maxValue: 1
    property bool muted: false
    property string iconName: ""
    property string label: ""

    readonly property string placement: root.item ? root.item.placement : "top"
    readonly property real gap: root.item ? root.item.gap : 0
    readonly property real reveal: root.item ? root.item.reveal : 0

    sourceComponent: Lithic.design === "material" ? material
        : Lithic.design === "adwaita" ? adwaita
        : Lithic.design === "fluent" ? fluent
        : lithic

    Component {
        id: lithic
        LithicOsd {
            kind: root.kind
            u: root.u
            shown: root.shown
            value: root.value
            maxValue: root.maxValue
            muted: root.muted
            iconName: root.iconName
            label: root.label
        }
    }
    Component {
        id: material
        MaterialOsd {
            kind: root.kind
            u: root.u
            shown: root.shown
            value: root.value
            maxValue: root.maxValue
            muted: root.muted
            iconName: root.iconName
            label: root.label
        }
    }
    Component {
        id: adwaita
        AdwaitaOsd {
            kind: root.kind
            u: root.u
            shown: root.shown
            value: root.value
            maxValue: root.maxValue
            muted: root.muted
            iconName: root.iconName
            label: root.label
        }
    }
    Component {
        id: fluent
        FluentOsd {
            kind: root.kind
            u: root.u
            shown: root.shown
            value: root.value
            maxValue: root.maxValue
            muted: root.muted
            iconName: root.iconName
            label: root.label
        }
    }
}
