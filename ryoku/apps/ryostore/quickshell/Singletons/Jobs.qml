pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui

// Ryostore's one package job (Ryoku.Ui's PackageJob, shared with Settings), held
// here so every card, row and catalogue reload reads the same run.
Singleton {
    readonly property alias app: job

    PackageJob { id: job }
}
