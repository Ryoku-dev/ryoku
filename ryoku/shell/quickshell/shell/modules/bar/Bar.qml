pragma ComponentBehavior: Bound

import QtQuick

Item {
    id: bar

    required property real railScale
    required property var frameBars
    required property var style
    required property string monitorName
    // Per-edge reveal baseline from the shared reveal model in shell.qml.
    required property var revealState

    signal menuRequested(string id, rect ownerRect)
    signal surfaceRequested(string id, rect ownerRect)
    signal actionRequested(string id)

    // A rail's drawn band thickness: its size when it is on and holds widgets,
    // else 0. Side rails inset by the top and bottom bands so they clear those
    // bands at the shared corners instead of overlapping their widgets.
    function bandOf(edge) {
        const r = bar.frameBars.rails[edge];
        if (!r || r.enabled !== true) return 0;
        const zs = edge === "top" || edge === "bottom" ? ["start", "center", "end"] : ["top", "center", "bottom"];
        let n = 0;
        for (const z of zs) n += (r[z] || []).length;
        return n > 0 ? r.size * bar.railScale : 0;
    }

    Repeater {
        model: ["top", "left", "bottom", "right"]

        FrameRail {
            required property string modelData
            edge: modelData
            revealed: bar.revealState ? bar.revealState[modelData] === true : false
            rail: bar.frameBars.rails[modelData]
            style: bar.style
            railScale: bar.railScale
            readonly property bool horiz: modelData === "top" || modelData === "bottom"
            leadInset: horiz ? 0 : bar.bandOf("top")
            tailInset: horiz ? 0 : bar.bandOf("bottom")
            delegate: Component {
                BarWidgetHost {
                    edge: modelData
                    // Fit the widget to the rail's band: a rail thinner than the
                    // 48px parity size scales its widgets down so the button and
                    // its hover highlight never overflow a small bar.
                    scale: bar.railScale * Math.min(1, bar.frameBars.rails[modelData].size / 48)
                    monitorName: bar.monitorName
                    onMenuRequested: (id, ownerRect) => bar.menuRequested(id, ownerRect)
                    onActionRequested: id => bar.actionRequested(id)
                }
            }
            visible: rail.enabled
        }
    }
}
