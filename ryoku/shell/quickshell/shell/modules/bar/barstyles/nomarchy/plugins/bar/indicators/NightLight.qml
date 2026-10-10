import QtQuick 6.11
import shell.services
import qs.Ui
import Ryoku.Ui.Singletons
BarIndicator {
  id: root

  active: Nightlight.on
  activeText: "󰔎"
  inactiveText: "󰔎"
  activeTooltipText: I18n.tr("Day Light")
  inactiveTooltipText: I18n.tr("Night Light")

  function toggle() {
    Nightlight.toggle()
  }

  onPressed: function() { root.toggle() }
}
