pragma ComponentBehavior: Bound

import QtQuick
import inir.modules.ii.overlay.crosshair
import inir.modules.ii.overlay.discord
import inir.modules.ii.overlay.floatingImage
import inir.modules.ii.overlay.fpsLimiter
import inir.modules.ii.overlay.gamePerformance
import inir.modules.ii.overlay.notes
import inir.modules.ii.overlay.notifications
import inir.modules.ii.overlay.resources
import inir.modules.ii.overlay.volumeMixer

DelegateChooser {
    role: "identifier"
    DelegateChoice { roleValue: "crosshair"; Crosshair {} }
    DelegateChoice { roleValue: "floatingImage"; FloatingImage {} }
    DelegateChoice { roleValue: "fpsLimiter"; FpsLimiter {} }
    DelegateChoice { roleValue: "resources"; Resources {} }
    DelegateChoice { roleValue: "gamePerformance"; GamePerformance {} }
    DelegateChoice { roleValue: "notes"; Notes {} }
    DelegateChoice { roleValue: "discord"; Discord {} }
    DelegateChoice { roleValue: "volumeMixer"; VolumeMixer {} }
    DelegateChoice { roleValue: "notifications"; Notifications {} }
}
