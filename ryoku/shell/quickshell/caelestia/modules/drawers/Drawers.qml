pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import caelestia.services

Variants {
    model: Screens.screens

    Scope {
        id: scope

        required property ShellScreen modelData

        Exclusions {
            screen: scope.modelData
            bar: content.bar
            visibilities: content.visibilities
        }

        ContentWindow {
            id: content

            screen: scope.modelData
        }
    }
}
