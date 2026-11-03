pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import qs.modules.common

Singleton {
    id: root

    property var matugen: ({})
    property var cavaGradient: []

    readonly property var themeColors: [
        root.colour(root.matugen.primary, Appearance.colors.colPrimary),
        root.colour(root.matugen.secondary, Appearance.colors.colSecondary),
        root.colour(root.matugen.tertiary, Appearance.colors.colTertiary)
    ]
    readonly property var cavaColors: root.cavaGradient.length
        ? root.cavaGradient
        : root.themeColors

    function colour(value, fallback) {
        const candidate = String(value ?? "")
        return /^#[0-9a-f]{6}([0-9a-f]{2})?$/i.test(candidate)
            ? Qt.color(candidate) : Qt.color(fallback)
    }

    function readMatugen() {
        try {
            const parsed = JSON.parse(paletteFile.text())
            if (typeof parsed === "object" && parsed !== null && /^#[0-9a-f]{6}$/i.test(String(parsed.primary ?? "")))
                root.matugen = parsed
        } catch (error) {
        }
    }

    function readCava() {
        const text = String(cavaFile.text() ?? "")
        const lines = text.split(/\r?\n/)
        let section = ""
        let gradient = false
        let foreground = ""
        const stops = []
        for (const line of lines) {
            const clean = line.trim()
            if (!clean || clean.startsWith(";") || clean.startsWith("#")) continue
            const header = clean.match(/^\[([^\]]+)\]$/)
            if (header) {
                section = header[1].toLowerCase()
                continue
            }
            if (section !== "color") continue
            const assignment = clean.match(/^([a-z0-9_]+)\s*=\s*['"]?(#[0-9a-f]{6}(?:[0-9a-f]{2})?)['"]?/i)
            if (assignment) {
                if (assignment[1] === "foreground") foreground = assignment[2]
                const stop = assignment[1].match(/^gradient_color_(\d+)$/)
                if (stop) stops.push([Number(stop[1]), assignment[2]])
            }
            if (/^gradient\s*=\s*1\s*$/.test(clean)) gradient = true
            if (/^gradient\s*=\s*0\s*$/.test(clean)) gradient = false
        }
        stops.sort((a, b) => a[0] - b[0])
        const colors = gradient ? stops.map(stop => stop[1]) : []
        if (colors.length === 0 && foreground) colors.push(foreground)
        if (colors.length > 0)
            root.cavaGradient = colors.map(color => root.colour(color, Appearance.colors.colPrimary))
    }

    FileView {
        id: paletteFile
        path: (Quickshell.env("HOME") || "") + "/.cache/ryoku/colors.json"
        watchChanges: true
        blockLoading: true
        printErrors: false
        onLoaded: root.readMatugen()
        onFileChanged: reload()
    }

    FileView {
        id: cavaFile
        path: (Quickshell.env("HOME") || "") + "/.config/cava/config"
        watchChanges: true
        blockLoading: true
        printErrors: false
        onLoaded: root.readCava()
        onFileChanged: reload()
    }

    Component.onCompleted: {
        root.readMatugen()
        root.readCava()
    }
}
