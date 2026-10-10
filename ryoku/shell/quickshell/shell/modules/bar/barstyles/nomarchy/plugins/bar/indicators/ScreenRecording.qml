import QtQuick 6.11
import shell.services
import qs.Ui
import Ryoku.Ui.Singletons
BarIndicator {
  id: root

  readonly property bool recording: Recorder.active

  active: recording
  activeText: "󰻂"
  inactiveText: "󰻂"
  activeTooltipText: I18n.tr("Stop recording")
  inactiveTooltipText: I18n.tr("Screen Recording")


  onPressed: function() {
    if (root.recording)
      Recorder.stop()
    else
      Spawn.run(["sh", "-c", "flock -n -o /tmp/ryoshot.lock qs -c ryoshot"])
  }
}
