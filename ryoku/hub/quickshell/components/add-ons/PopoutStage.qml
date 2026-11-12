import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a small screen showing where a pop-out opens; the middle third is the centre, elsewhere the nearest edge
Item {
    id: stage

    property string edge: "top"
    property string align: "start"
    signal placed(string edge, string align)

    readonly property real screenWidth: Math.min(stage.width, 360)
    readonly property real screenHeight: Math.round(stage.screenWidth / 1.6)
    readonly property bool centred: stage.edge === "center"
    readonly property bool upright: stage.edge === "left" || stage.edge === "right"
    readonly property real inset: LithicTokens.space.s2

    implicitHeight: stage.screenHeight + LithicTokens.space.s2 + hint.implicitHeight

    Label {
        id: hint
        y: stage.screenHeight + LithicTokens.space.s2
        width: stage.width
        text: I18n.tr("Click a spot on the screen, or drag the pop-out, to choose where it opens.")
        typeRole: "detail"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    function third(f) {
        return f < 1 / 3 ? "start" : f < 2 / 3 ? "center" : "end";
    }

    function edgeAt(fx, fy) {
        if (stage.third(fx) === "center" && stage.third(fy) === "center")
            return "center";
        var d = { left: fx, right: 1 - fx, top: fy, bottom: 1 - fy };
        var best = "top";
        for (var e in d)
            if (d[e] < d[best])
                best = e;
        return best;
    }

    function alignAt(edge, fx, fy) {
        if (edge === "center")
            return stage.align;
        return edge === "left" || edge === "right" ? stage.third(fy) : stage.third(fx);
    }

    function along(span, size) {
        return stage.align === "start" ? stage.inset
            : stage.align === "end" ? span - size - stage.inset
            : (span - size) / 2;
    }

    function chipX() {
        return stage.centred ? (screen.width - chip.width) / 2
            : stage.edge === "left" ? stage.inset
            : stage.edge === "right" ? screen.width - chip.width - stage.inset
            : stage.along(screen.width, chip.width);
    }

    function chipY() {
        return stage.centred ? (screen.height - chip.height) / 2
            : stage.edge === "top" ? stage.inset
            : stage.edge === "bottom" ? screen.height - chip.height - stage.inset
            : stage.along(screen.height, chip.height);
    }

    function drop(px, py) {
        var fx = Math.max(0, Math.min(1, px / screen.width));
        var fy = Math.max(0, Math.min(1, py / screen.height));
        var e = stage.edgeAt(fx, fy);
        stage.placed(e, stage.alignAt(e, fx, fy));
    }

    CutRect {
        id: screen
        width: stage.screenWidth
        height: stage.screenHeight
        cut: LithicTokens.cut.small
        fill: Lithic.field
        strokeColor: Lithic.edge
        strokeWidth: LithicTokens.size.hairline

        Rectangle {
            x: screen.width / 3
            y: screen.height / 3
            width: screen.width / 3
            height: screen.height / 3
            color: stage.centred ? Lithic.alpha(Lithic.fg, 0.06) : "transparent"
            border.width: LithicTokens.size.hairline
            border.color: Lithic.vein
        }

        Label {
            visible: !stage.centred
            anchors.centerIn: parent
            text: I18n.tr("Centre")
            typeRole: "caption"
            inkRole: "fg3"
        }

        MouseArea {
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            onClicked: mouse => stage.drop(mouse.x, mouse.y)
        }

        CutRect {
            id: chip
            width: stage.upright ? Math.round(screen.width * 0.18) : Math.round(screen.width * 0.3)
            height: stage.upright ? Math.round(screen.height * 0.48) : Math.round(screen.height * 0.3)
            x: stage.chipX()
            y: stage.chipY()
            cut: LithicTokens.cut.token
            fill: Lithic.sel

            Label {
                anchors.centerIn: parent
                width: parent.width - LithicTokens.space.s2
                horizontalAlignment: Text.AlignHCenter
                text: I18n.tr("Pop-out")
                typeRole: "caption"
                ink: Lithic.onSel
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.OpenHandCursor
                drag.target: chip
                drag.minimumX: 0
                drag.maximumX: screen.width - chip.width
                drag.minimumY: 0
                drag.maximumY: screen.height - chip.height
                onReleased: {
                    var px = chip.x + chip.width / 2, py = chip.y + chip.height / 2;
                    chip.x = Qt.binding(() => stage.chipX());
                    chip.y = Qt.binding(() => stage.chipY());
                    stage.drop(px, py);
                }
            }
        }
    }
}
