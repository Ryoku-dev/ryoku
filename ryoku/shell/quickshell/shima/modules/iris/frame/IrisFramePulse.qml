pragma ComponentBehavior: Bound

import QtQuick
import qs.services
import qs.modules.common
import qs.modules.common.widgets
import qs.modules.iris.style
import qs.modules.iris.frame

Item {
    id: root

    property bool active: false
    readonly property var surround: Config.options?.iris?.surround ?? ({})
    readonly property string edges: String(root.surround?.musicEdges ?? "sides")
    readonly property real strength: Math.max(0.5, Math.min(3, Number(root.surround?.musicStrength ?? 160) / 100))
    readonly property real sensitivity: Math.max(0.5, Math.min(2.5, Number(root.surround?.musicSensitivity ?? 140) / 100))
    readonly property real speed: Math.max(0.4, Math.min(2, Number(root.surround?.musicSpeed ?? 100) / 100))
    readonly property real reach: 18 * IrisStyle.density * root.strength
    property real level: 0
    property real lowLevel: 0
    property real midLevel: 0
    property real highLevel: 0
    property real phase: 0
    readonly property real visibleLevel: root.active ? root.level : 0
    readonly property vector4d amplitudes: Qt.vector4d(
        root.edges !== "horizontal" ? (root.lowLevel * 0.6 + root.midLevel * 0.4) * root.reach : 0,
        root.edges !== "sides" ? (root.highLevel * 0.75 + root.midLevel * 0.25) * root.reach : 0,
        root.edges !== "horizontal" ? (root.midLevel * 0.6 + root.highLevel * 0.4) * root.reach : 0,
        root.edges !== "sides" ? (root.lowLevel * 0.7 + root.midLevel * 0.3) * root.reach : 0)
    onActiveChanged: {
        if (!root.active) {
            root.level = 0
            root.lowLevel = 0
            root.midLevel = 0
            root.highLevel = 0
            root.phase = 0
        }
    }

    CavaProcess {
        id: cava
        active: root.active
        sampleCount: 16
    }
    FrameAnimation {
        running: root.active && (cava.audioSignalActive || root.level > 0.002)
        onTriggered: {
            const points = cava.points ?? []
            const step = Math.min(0.05, frameTime)
            const ceiling = Math.max(1, cava.normalizationCeiling)
            function band(start, end) {
                if (!points.length) return 0
                const from = Math.floor(points.length * start)
                const to = Math.max(from + 1, Math.min(points.length, Math.ceil(points.length * end)))
                let sum = 0
                for (let i = from; i < to; ++i) sum += Math.max(0, Number(points[i] ?? 0))
                return Math.min(1, Math.sqrt(Math.min(1, sum / (to - from) / ceiling)) * root.sensitivity)
            }
            function smooth(current, target) {
                const rate = target > current ? 18 : 3.2
                const result = current + (target - current) * Math.min(1, rate * step)
                return result < 0.002 && !cava.audioSignalActive ? 0 : result
            }
            root.lowLevel = smooth(root.lowLevel, band(0, 0.25))
            root.midLevel = smooth(root.midLevel, band(0.25, 0.68))
            root.highLevel = smooth(root.highLevel, band(0.68, 1))
            root.level = Math.max(root.lowLevel, root.midLevel, root.highLevel)
            root.phase = (root.phase + step * (0.6 + 2.4 * root.level) * root.speed) % 6283.185
        }
    }
}
