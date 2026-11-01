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

    property bool present: root.shown.length > 0

    // The window the bar shows: the 5 h or weekly one the Modules page picked
    // (the other when the chosen one has no data), or whichever is fullest.
    readonly property var lead: {
        const choice = S.Prefs.aiWindow;
        const groups = choice === "closest" ? [[false, true]]
            : choice === "weekly" ? [[true], [false]] : [[false], [true]];
        for (const group of groups) {
            let best = null;
            for (const id of root.shown) {
                const tool = AiUsage.tool(id);
                for (const weekly of group) {
                    if (!(weekly ? tool.has7d : tool.has5h))
                        continue;
                    const used = weekly ? tool.pct7d : tool.pct5h;
                    if (best === null || used > best.used) {
                        best = { id: id, weekly: weekly, used: used,
                            pace: AiUsage.pace(id, weekly) };
                    }
                }
            }
            if (best !== null)
                return best;
        }
        return null;
    }
    readonly property bool pacing: S.Prefs.aiPace && root.lead !== null
        && root.lead.pace.available
    readonly property string trend: {
        if (!root.pacing)
            return "";
        const delta = root.lead.pace.delta;
        return delta > S.Prefs.aiTolerance ? "ahead"
            : delta < -S.Prefs.aiTolerance ? "behind" : "even";
    }
    readonly property bool hot: root.shown.some(id => AiUsage.tool(id).blocked
        || root.windowHot(id, false) || root.windowHot(id, true))

    // Past the alert level, or burning fast enough to fill before the reset.
    function windowHot(id, weekly) {
        const tool = AiUsage.tool(id);
        if (!(weekly ? tool.has7d : tool.has5h))
            return false;
        if ((weekly ? tool.pct7d : tool.pct5h) >= S.Prefs.aiAlertAt)
            return true;
        if (!S.Prefs.aiPace)
            return false;
        const pace = AiUsage.pace(id, weekly);
        return pace.available && pace.exhaustTs > 0
            && pace.delta > S.Prefs.aiTolerance;
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
        if (!S.Prefs.aiPace || !pace.available)
            return "";
        const points = Math.abs(pace.delta);
        let text = points <= S.Prefs.aiTolerance ? "on pace"
            : points + " pts " + (pace.delta > 0 ? "ahead of" : "behind") + " pace";
        if (pace.exhaustTs > 0 && pace.delta > S.Prefs.aiTolerance)
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
        const paceText = root.paceText(AiUsage.pace(id, weekly));
        const detail = [resetText.length > 0 ? "resets " + resetText : "",
            paceText].filter(part => part.length > 0).join(", ");
        return label + " " + percent + "%"
            + (detail.length > 0 ? " (" + detail + ")" : "");
    }

    function figure() {
        if (root.lead === null)
            return "-";
        if (root.pacing && S.Prefs.aiFigure === "pace") {
            const delta = root.lead.pace.delta;
            return (delta > 0 ? "+" : delta < 0 ? "−" : "") + Math.abs(delta);
        }
        return root.lead.used + "%";
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
}
