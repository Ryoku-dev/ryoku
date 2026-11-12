pragma Singleton

import QtQuick
import Quickshell
import shell.services as Svc

// Desktop backdrop motion. Only the wallpaper swap is animated: a 200 ms linear
// crossfade on each revision (contract 08 sec 5, TRANSITION_DURATION_MS), the
// shell's own Motion.wallpaperFade, so Speed, reduce motion and low power
// retime it like every other shell animation.
Singleton {
    readonly property bool reduce: Svc.Motion.reduce
    readonly property int wallpaperFade: Svc.Motion.wallpaperFade

}
