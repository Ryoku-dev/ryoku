import QtQuick
import Ryoku.Ui.Singletons
import shell.services
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane

    // Tools the Modules page left on; a tool with no usage data stays hidden
    // either way.
    readonly property var shown: AiUsage.present.filter(
        id => S.Prefs.aiTools.indexOf(id) >= 0)

    property real nowSec: Date.now() / 1000
    readonly property int paceTolerance: 10

    property bool present: root.shown.length > 0

    readonly property var lead: {
        let best = null;
        for (const id of root.shown) {
            const tool = AiUsage.tool(id);
            if (tool.has5h && (best === null || tool.pct5h > best.used)) {
                best = { id: id, weekly: false, used: tool.pct5h,
                    pace: AiUsage.pace(id, false, root.nowSec) };
            }
        }
        return best;
    }
    readonly property bool pacing: root.lead !== null
        && root.lead.pace.available
    readonly property string trend: {
        if (!root.pacing)
            return "";
        const delta = root.lead.pace.delta;
        return delta > root.paceTolerance ? "ahead"
            : delta < -root.paceTolerance ? "behind" : "even";
    }
    readonly property bool hot: root.shown.some(id => AiUsage.tool(id).blocked
        || root.windowHot(id, false) || root.windowHot(id, true))

    function windowHot(id, weekly) {
        const tool = AiUsage.tool(id);
        if (!(weekly ? tool.has7d : tool.has5h))
            return false;
        if ((weekly ? tool.pct7d : tool.pct5h) >= S.Prefs.aiAlertAt)
            return true;
        const pace = AiUsage.pace(id, weekly, root.nowSec);
        return pace.available && pace.exhaustTs > 0
            && pace.delta > root.paceTolerance;
    }

    function clock(timestamp) {
        if (!(timestamp > 0))
            return "";
        const date = new Date(timestamp * 1000);
        const sameDay = date.toDateString() === new Date().toDateString();
        return (sameDay ? "" : date.toLocaleDateString(TimeFormat.locale, "ddd") + " ")
            + TimeFormat.format(date);
    }

    function paceText(pace) {
        if (!pace.available)
            return "";
        const points = Math.abs(pace.delta);
        let text = points <= root.paceTolerance ? "on pace"
            : points + " pts " + (pace.delta > 0 ? "ahead of" : "behind") + " pace";
        if (pace.exhaustTs > 0 && pace.delta > root.paceTolerance)
            text += ", full ~" + root.clock(pace.exhaustTs);
        return text;
    }

    function windowText(id, weekly) {
        const tool = AiUsage.tool(id);
        const available = weekly ? tool.has7d : tool.has5h;
        const label = weekly ? "weekly" : "5 h";
        if (!available)
            return label + " unavailable";
        const percent = weekly ? tool.pct7d : tool.pct5h;
        const reset = weekly ? tool.reset7d : tool.reset5h;
        const resetText = root.clock(reset);
        const paceText = root.paceText(AiUsage.pace(id, weekly, root.nowSec));
        const detail = [resetText.length > 0 ? "resets " + resetText : "",
            paceText].filter(part => part.length > 0).join(", ");
        return label + " " + percent + "%"
            + (detail.length > 0 ? " (" + detail + ")" : "");
    }

    function figure() {
        return root.lead === null ? "-" : root.lead.used + "%";
    }

    readonly property string hintText: root.shown.map(id => {
        const tool = AiUsage.tool(id);
        const name = AiUsage.labels[id] || id;
        const state = tool.blocked ? "limit reached, " : "";
        return name + " " + state + root.windowText(id, false)
            + ", " + root.windowText(id, true);
    }).join(" · ")

    implicitWidth: present ? body.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Row {
        id: body
        anchors.centerIn: parent
        spacing: K.Style.s1
        scale: press.pressScale

        K.Glyph {
            anchors.verticalCenter: parent.verticalCenter
            name: "auto_awesome"
            size: K.Style.barIcon
            tone: root.hot ? "alert" : "dim"
        }

        K.Label {
            anchors.verticalCenter: parent.verticalCenter
            text: root.figure()
            role: "bar"
            numeric: true
            strong: true
        }

        K.Glyph {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.trend === "ahead" || root.trend === "behind"
            name: root.trend === "ahead" ? "trending_up" : "trending_down"
            size: Math.round(K.Style.barIcon * 0.8)
            tone: root.trend === "ahead" ? "alert" : "dim"
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: AiUsage.refresh()
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: root.present && press.hovered && root.hintText.length > 0
    }

    Timer {
        interval: 30000
        repeat: true
        running: root.present && root.visible && !root.scene.autoHidden
        onRunningChanged: if (running) root.nowSec = Date.now() / 1000
        onTriggered: root.nowSec = Date.now() / 1000
    }
}
