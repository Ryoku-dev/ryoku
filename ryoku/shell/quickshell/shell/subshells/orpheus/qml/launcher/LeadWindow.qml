import QtQuick
import Ryoku.Ui.Singletons

// An open window: its app icon, its title, the workspace it sits on, Focus.
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

    subtitle: root.workspace.length > 0 ? I18n.tr("Workspace %1").arg(root.workspace)
        : root.row ? String(root.row.subtitle || "") : ""
}
