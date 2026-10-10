import QtQuick 6.11
import QtQuick.Layouts
import Ryoku.Ui.Singletons
import qs.Commons
import qs.Ui

BarWidget {
  id: root
  moduleName: "omarchy.workspaces"

  readonly property var workspaceRows: {
    var live = Wm.workspaces || []
    if (live.length > 0) return live
    var fallback = []
    for (var i = 1; i <= 5; i++)
      fallback.push({ id: String(i), name: String(i), active: false, occupied: false, canActivate: true })
    return fallback
  }

  function focusWorkspace(workspace) {
    if (workspace && workspace.canActivate !== false)
      Wm.focusWorkspace(workspace.id)
  }

  readonly property real trailingGap: root.vertical ? 0 : Style.spaceReal(1.5)

  implicitWidth: grid.implicitWidth + trailingGap
  implicitHeight: grid.implicitHeight

  GridLayout {
    id: grid
    anchors.fill: parent
    anchors.rightMargin: root.trailingGap
    columns: root.vertical ? 1 : root.workspaceRows.length
    columnSpacing: root.vertical ? 0 : Style.space(1)
    rowSpacing: root.vertical ? Style.space(2) : 0

    Repeater {
      model: root.workspaceRows

      WidgetButton {
        required property var modelData

        readonly property bool occupied: modelData.occupied === true
        readonly property bool focused: modelData.active === true

        bar: root.bar
        text: focused ? "\uDB85\uDCFB" : String(modelData.name || modelData.id)
        opacity: occupied || focused ? 1 : 0.5
        horizontalMargin: 6
        verticalPadding: 6
        fixedWidth: root.vertical ? root.barSize : Style.space(20)
        fixedHeight: root.barSize
        enabled: modelData.canActivate !== false
        onPressed: function() { root.focusWorkspace(modelData) }
      }
    }
  }
}
