import QtQuick 6.11
import shell.services
import qs.Ui
import Ryoku.Ui.Singletons
BarIndicator {
  id: root

  active: Flags.keepAwake
  activeText: "󰅶"
  inactiveText: "󰅶"
  activeTooltipText: I18n.tr("Allow Idle Lock & Screensaver")
  inactiveTooltipText: I18n.tr("Stay Awake")

  function toggle() {
    Flags.keepAwake = !Flags.keepAwake
  }

  onPressed: function() { root.toggle() }
}
