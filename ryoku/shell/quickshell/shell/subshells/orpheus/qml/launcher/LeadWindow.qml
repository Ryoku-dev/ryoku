import QtQuick
import Ryoku.Ui.Singletons
import "lib/rows.js" as Rows

// An open window: its app icon, its title, the workspace it sits on, what its
// program holds in memory, then Focus and Kill.
LeadGeneric {
    id: root

    readonly property string workspace: {
        const address = root.row ? String(root.row.windowAddress || "") : "";
        const list = Wm.windows;
        for (let i = 0; i < list.length; i++)
            if (String(list[i].id) === address)
                return root.lead ? root.lead.workspaceLabel(list[i].workspace) : "";
        return "";
    }
    readonly property var info: root.engine && root.row
        ? root.engine.windowInfo[String(root.row.windowAddress || "")] || null : null
    readonly property string memory: root.info ? Rows.memoryLabel(root.info.bytes) : ""

    subtitle: root.workspace.length > 0 ? I18n.tr("Workspace %1").arg(root.workspace)
        : root.row ? String(root.row.subtitle || "") : ""
    description: {
        if (root.memory.length === 0)
            return "";
        if (root.info.shared > 1)
            return I18n.tr("%1 in memory, shared by %2 windows of this program").arg(root.memory).arg(root.info.shared);
        return I18n.tr("%1 in memory").arg(root.memory);
    }
}
