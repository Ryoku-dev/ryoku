pragma Singleton

import ".."
import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.services
import caelestia.utils

Searcher {
    id: root

    function transformSearch(search: string): string {
        return search.slice(`${GlobalConfig.launcher.actionPrefix}variant `.length);
    }

    useFuzzy: GlobalConfig.launcher.useFuzzy.variants
    list: [
        Variant {
            variant: "vibrant"
            icon: "sentiment_very_dissatisfied"
            name: qsTr("Vibrant")
            description: qsTr("A high chroma palette. The primary palette's chroma is at maximum.")
        },
        Variant {
            variant: "tonalspot"
            icon: "android"
            name: qsTr("Tonal Spot")
            description: qsTr("Default for Material theme colors. A pastel palette with a low chroma.")
        },
        Variant {
            variant: "expressive"
            icon: "compare_arrows"
            name: qsTr("Expressive")
            description: qsTr("A medium chroma palette. The primary palette's hue is different from the seed color, for variety.")
        },
        Variant {
            variant: "fidelity"
            icon: "compare"
            name: qsTr("Fidelity")
            description: qsTr("Matches the seed color, even if the seed color is very bright (high chroma).")
        },
        Variant {
            variant: "content"
            icon: "sentiment_calm"
            name: qsTr("Content")
            description: qsTr("Almost identical to fidelity.")
        },
        Variant {
            variant: "fruitsalad"
            icon: "nutrition"
            name: qsTr("Fruit Salad")
            description: qsTr("A playful theme - the seed color's hue does not appear in the theme.")
        },
        Variant {
            variant: "rainbow"
            icon: "looks"
            name: qsTr("Rainbow")
            description: qsTr("A playful theme - the seed color's hue does not appear in the theme.")
        },
        Variant {
            variant: "neutral"
            icon: "contrast"
            name: qsTr("Neutral")
            description: qsTr("Close to grayscale, a hint of chroma.")
        },
        Variant {
            variant: "monochrome"
            icon: "filter_b_and_w"
            name: qsTr("Monochrome")
            description: qsTr("All colors are grayscale, no chroma.")
        }
    ]

    component Variant: QtObject {
        required property string variant
        required property string icon
        required property string name
        required property string description

        function onClicked(list: var): void {
            if (list) {
                list.visibilities.launcher = false;
            }
            // the desktop's matugen scheme type, which shapes the palette while the
            // theme follows the wallpaper
            const types = { tonalspot: "TonalSpot", fruitsalad: "FruitSalad" };
            const type = types[variant] || variant.charAt(0).toUpperCase() + variant.slice(1);
            Quickshell.execDetached(["ryoku-shell", "call", "settings.patch", JSON.stringify({ path: "theme.matugen.scheme_type", value: type })]);
        }
    }
}
