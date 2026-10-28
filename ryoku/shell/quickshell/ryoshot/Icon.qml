import QtQuick
import QtQuick.Shapes
import "Singletons"

Item {
    id: icon

    property string name: ""
    property color tint: Theme.inkDim
    property real size: 18

    readonly property real vb: 24

    readonly property var defs: ({
        "select":  "M4 3l7 17 2-7 7-2z",
        "rect":    "M5 6h14a1 1 0 0 1 1 1v10a1 1 0 0 1 -1 1H5a1 1 0 0 1 -1 -1V7a1 1 0 0 1 1 -1z",
        "ellipse": "M20 12a8 6 0 0 1 -16 0a8 6 0 0 1 16 0z",
        "line":    "M5 19L19 5",
        "arrow":   "M5 19L19 5M19 5h-7M19 5v7",
        "pen":     "M4 18c3-1 4-6 8-9s6-4 8-5c-1 2-2 5-5 8s-8 5-9 8z",
        "text":    "M5 6h14M12 6v13",
        "marker":  "M4 15l10-10 4 4-10 10H4z",
        "blur":    "M11 9a2 2 0 0 1 -4 0a2 2 0 0 1 4 0z M17 13a2 2 0 0 1 -4 0a2 2 0 0 1 4 0z M10.5 16a1.5 1.5 0 0 1 -3 0a1.5 1.5 0 0 1 3 0z",
        "undo":    "M9 7L4 12l5 5M4 12h11a5 5 0 0 1 0 10",
        "redo":    "M15 7l5 5-5 5M20 12H9a5 5 0 0 0 0 10",
        "copy":    "M11 9h9a2 2 0 0 1 2 2v9a2 2 0 0 1 -2 2h-9a2 2 0 0 1 -2 -2v-9a2 2 0 0 1 2 -2z M5 15V5a2 2 0 0 1 2 -2h10",
        "save":    "M5 3h12l4 4v14H5zM8 3v6h8M8 21v-7h8v7",
        "upload":  "M12 16V4M7 9l5-5 5 5M5 20h14",
        "cancel":  "M6 6l12 12M18 6L6 18",
        "gear":    "M12 8.5a3.5 3.5 0 0 1 0 7a3.5 3.5 0 0 1 0 -7z M12 2.5l1.4 2.2 2.6-.5.4 2.6 2.3 1.3-1.1 2.4 1.1 2.4-2.3 1.3-.4 2.6-2.6-.5L12 21.5l-1.4-2.2-2.6.5-.4-2.6-2.3-1.3 1.1-2.4-1.1-2.4 2.3-1.3.4-2.6 2.6.5z",
        "counter": "M12 3.6a8.4 8.4 0 1 1 -0.1 0z M11 9.2l1.7 -1.1V16",
        "pixelate": "M4 4h6v6H4z M14 4h6v6h-6z M4 14h6v6H4z M14 14h6v6h-6z",
        "sketch":  "M4 16c1.6 -3.4 3.2 3.4 4.8 0s3.2 3.4 4.8 0 3.2 3.4 4.4 1",
        "magnify": "M10 4a6 6 0 1 0 0.01 0z M14.4 14.4L21 21",
        "redact":    "M4 7h16v4H4z M4 14h3v3H4z M10 14h3v3h-3z M16 14h3v3h-3z",
        "spotlight": "M12 6a6 6 0 1 0 0.01 0z M3 7l2.5 1 M7 3l1 2.5",
        "ocr":       "M3 6V3h3 M18 3h3v3 M3 18v3h3 M21 18v3h-3 M9 17l3 -8 3 8 M10.2 14h3.6",
        "pin":       "M12 7.5a3.5 3.5 0 1 0 -7 0a3.5 3.5 0 0 0 7 0z M11 10L18 18",
        "fill":      "M4 4h16v16H4z M4 4L20 20 M4 8L16 20 M4 12L12 20 M4 16L8 20",
        "check":     "M5 13l4 4L19 6",
        "camera":    "M3.2 8.6a1.6 1.6 0 0 1 1.6 -1.6h2l1.3 -2h6.4l1.3 2h2a1.6 1.6 0 0 1 1.6 1.6v8.8a1.6 1.6 0 0 1 -1.6 1.6H4.8a1.6 1.6 0 0 1 -1.6 -1.6z M15.5 13a3.5 3.5 0 1 1 -7 0a3.5 3.5 0 0 1 7 0z",
        "video":     "M3.5 7.5a1 1 0 0 1 1 -1h9a1 1 0 0 1 1 1v9a1 1 0 0 1 -1 1h-9a1 1 0 0 1 -1 -1z M14.5 10.5l6 -3.5v10l-6 -3.5z",
        "eyedropper":"M20.4 3.6a2 2 0 0 0 -2.8 0l-2.3 2.3 2.8 2.8 2.3 -2.3a2 2 0 0 0 0 -2.8z M14.3 7.2L5 16.5 4 20l3.5 -1 9.3 -9.3z",
        "fullscreen":"M4 9V4h5 M15 4h5v5 M20 15v5h-5 M9 20H4v-5",
        "sound":     "M4 9.5h3l4 -3.5v12l-4 -3.5H4z M15 9a3.5 3.5 0 0 1 0 6 M17.5 6.5a7 7 0 0 1 0 11",
        "timer":     "M9 3h6 M12 3v3 M17.5 6.5l1.5 -1.5 M12 7a7 7 0 1 1 -0.01 0z M12 10v3l2 2",
        "mic":       "M9 5a3 3 0 0 1 6 0v7a3 3 0 0 1 -6 0z M6 12a6 6 0 0 0 12 0 M12 18v3 M9 21h6",
        "keyboard":  "M3 6h18v12H3z M6 9h1 M10 9h1 M14 9h1 M18 9h1 M6 12h1 M10 12h1 M14 12h1 M18 12h1 M7 15h10",
        "webcam":    "M5 7h14v10H5z M15 12a3 3 0 1 1 -6 0a3 3 0 0 1 6 0z M12 17v3 M9 20h6",
        "folder":    "M3 6h7l2 2h9v11H3z",
        "phone":     "M6 2h12a2 2 0 0 1 2 2v16a2 2 0 0 1 -2 2H6a2 2 0 0 1 -2 -2V4a2 2 0 0 1 2 -2z M11 5h2 M12 18h.01"
    })

    readonly property string d: defs[name] !== undefined ? defs[name] : ""

    Shape {
        anchors.centerIn: parent
        width: icon.vb
        height: icon.vb
        scale: icon.size / icon.vb
        preferredRendererType: Shape.CurveRenderer
        antialiasing: true

        ShapePath {
            strokeColor: icon.tint
            strokeWidth: 1.7
            fillColor: "transparent"
            capStyle: ShapePath.RoundCap
            joinStyle: ShapePath.RoundJoin
            PathSvg { path: icon.d }
        }
    }
}
