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

    readonly property string leadId: {
        let bestId = "";
        let highest = -1;
        for (const id of root.shown) {
            const tool = AiUsage.tool(id);
            if (tool.has5h && tool.pct5h > highest) {
                highest = tool.pct5h;
                bestId = id;
            }
        }
        return bestId;
    }
    readonly property var lead: root.leadId.length > 0
        ? AiUsage.tool(root.leadId) : null
    readonly property bool hot: root.shown.some(id => {
        const tool = AiUsage.tool(id);
        return tool.blocked || (tool.has5h && tool.pct5h >= 80)
            || (tool.has7d && tool.pct7d >= 80);
    })

    function clock(timestamp) {
        if (!(timestamp > 0))
            return "";
        const date = new Date(timestamp * 1000);
        const sameDay = date.toDateString() === new Date().toDateString();
        return (sameDay ? "" : date.toLocaleDateString(TimeFormat.locale, "ddd") + " ")
            + TimeFormat.format(date);
    }

    function windowText(tool, weekly) {
        const available = weekly ? tool.has7d : tool.has5h;
        const label = weekly ? "weekly" : "5 h";
        if (!available)
            return label + " unavailable";
        const percent = weekly ? tool.pct7d : tool.pct5h;
        const reset = weekly ? tool.reset7d : tool.reset5h;
        const resetText = root.clock(reset);
        return label + " " + percent + "%"
            + (resetText.length > 0 ? " (resets " + resetText + ")" : "");
    }

    readonly property string hintText: root.shown.map(id => {
        const tool = AiUsage.tool(id);
        const name = AiUsage.labels[id] || id;
        const state = tool.blocked ? "limit reached, " : "";
        return name + " " + state + root.windowText(tool, false)
            + ", " + root.windowText(tool, true);
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
            text: root.lead ? root.lead.pct5h + "%" : "—"
            role: "bar"
            numeric: true
            strong: true
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
