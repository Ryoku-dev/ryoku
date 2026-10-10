import QtQuick 6.11
import Quickshell.Io

// An IpcHandler the shell can also answer over its own socket (see
// IpcRegistry), so omarchy-shell reaches it without a qs ipc client. During a
// plugin reload the replacement claims its target before enabling itself,
// which keeps Quickshell from registering two native handlers at once.
IpcHandler {
  id: handler

  property bool wantsEnabled: true
  property bool ownsTarget: false
  property bool registrationReady: false

  enabled: wantsEnabled && ownsTarget && target !== ""

  Component.onCompleted: {
    registrationReady = true
    IpcRegistry.register(handler)
  }
  Component.onDestruction: IpcRegistry.unregister(handler)
  onTargetChanged: if (registrationReady) IpcRegistry.claim(handler)
  onWantsEnabledChanged: if (registrationReady) IpcRegistry.claim(handler)
}
