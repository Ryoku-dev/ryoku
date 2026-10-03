pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.SidebarFx 1.0

// The C++ depth edge, isolated behind a Loader so a shell still runs (on the
// chrome's gradient fallback) when the Ryoku.SidebarFx module is not built.
// The host binds width/height/progress/side/intensity/radius.
DepthEdge {
}
