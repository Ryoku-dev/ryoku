// Nomarchy hosts Omarchy's shell as a Ryoku bar style. The visual and plugin
// contracts track basecamp/omarchy quattro at d9b970dd; desktop state and
// actions cross Ryoku's neutral window-manager seam.

pragma ComponentBehavior: Bound

import QtQuick 6.11
import Quickshell
import Quickshell.Io
import qs.Commons
import "../../../../services/lib/screens.js" as Screens

Item {
  id: scene

  property var modelData: null
  readonly property string omarchyPath: (Quickshell.env("XDG_DATA_HOME")
    || Quickshell.env("HOME") + "/.local/share") + "/ryoku/nomarchy"
  readonly property string ipcSocketPath: Quickshell.env("XDG_RUNTIME_DIR") + "/omarchy-shell-"
    + Qt.md5(scene.omarchyPath + "/shell\n" + Quickshell.env("WAYLAND_DISPLAY")).slice(0, 16) + ".sock"

  width: 0
  height: 0

  readonly property bool isPrimary: {
    const screens = Screens.uniqueByName(Quickshell.screens)
    return screens.length > 0 && !!scene.modelData
      && screens[0].name === scene.modelData.name
  }

  Loader {
    active: scene.isPrimary
    source: active ? Qt.resolvedUrl("Host.qml") : ""
  }

  // The transport belongs to the stable style scene, not the reloadable
  // plugin host. Plugin rescans can replace every dynamic component without
  // taking the command socket down.
  SocketServer {
    active: scene.isPrimary
    path: scene.ipcSocketPath

    handler: Socket {
      id: connection

      parser: SplitParser {
        splitMarker: "\u001e"
        onRead: function(data) {
          var fields = String(data).split("\u001f")
          var result = fields.length >= 2
            ? IpcRegistry.call(fields[0], fields[1], fields.slice(2))
            : { ran: false }
          connection.write(result.ran
            ? "OK\u001f" + result.output + "\u001e"
            : "SKIP\u001e")
          connection.flush()
          connection.connected = false
        }
      }
    }
  }
}
