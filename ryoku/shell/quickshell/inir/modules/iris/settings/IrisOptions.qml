pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import inir.modules.common
import inir.services
import inir.modules.iris.style
import inir.modules.iris.pieces
import inir.modules.iris.control
import inir.modules.iris.background
import inir.modules.background.widgets

QtObject {
    id: root

    function plain(value: var): var {
        if (value === null || value === undefined || typeof value !== "object") return value
        if (value.length !== undefined && typeof value !== "string") return Array.from(value, item => root.plain(item))
        const out = {}
        for (const key of Object.keys(value)) out[key] = root.plain(value[key])
        return out
    }
    function same(a: var, b: var): bool {
        return JSON.stringify(root.plain(a)) === JSON.stringify(root.plain(b))
    }
    // An icon row is keyed by its piece inside one shared list, not by its own path.
    function currentValue(spec: var): var {
        if (spec.nightLight === "on") return Nightlight.active
        if (spec.nightLight === "warmth") return Nightlight.temperature
        if (spec.nightLight === "schedule") return Nightlight.scheduled
        if (spec.widgetDesign) return DesktopWidgetDesign.current
        if (spec.kind === "icon") return IrisPieces.chosenGlyph(String(spec.piece ?? ""))
        // A floating bubble the Island's list carries reads as off: it is on the Island (IrisPieces.listedOnIsland).
        if (/^iris\.bubbles\.extras\.\w+\.enable$/.test(String(spec.path ?? "")) && IrisPieces.listedOnIsland(String(spec.path).split(".")[3]))
            return false
        if (spec.bundle) {
            if (spec.keyed) return String(Config.getNestedValue(spec.bundle[0], "") ?? "")
            const now = spec.bundle.map(path => String(Config.getNestedValue(path, "") ?? ""))
            return (spec.choices ?? []).find(choice => root.same(choice.values.map(value => String(value)), now))?.value ?? ""
        }
        return Config.getNestedValue(spec.path, spec.fallback)
    }
    function commit(spec: var, next: var): void {
        // Night Light belongs to Ryoku's daemon; these rows drive it directly.
        if (spec.nightLight === "on") Nightlight.setActive(Boolean(next))
        else if (spec.nightLight === "warmth") Nightlight.setTemperature(Math.round(Number(next)))
        else if (spec.nightLight === "schedule") Nightlight.setScheduled(Boolean(next))
        else if (spec.widgetDesign) DesktopWidgetDesign.apply(String(next))
        else if (spec.kind === "icon") IrisPieces.setGlyph(String(spec.piece ?? ""), String(next ?? ""))
        else if (spec.bundle) {
            const choice = (spec.choices ?? []).find(candidate => candidate.value === next)
            if (choice) {
                const updates = {}
                spec.bundle.forEach((path, index) => updates[path] = choice.values[index])
                Config.setNestedValues(updates)
            }
        }
        else if (spec.path === "iris.bar.pieces") {
            // As a drag onto the Island does (IrisStage.place): a piece added here leaves its floating place.
            const updates = { "iris.bar.pieces": Array.from(next ?? []) }
            const before = Array.from(Config.options?.iris?.bar?.pieces ?? []).map(entry => String(entry))
            for (const id of Array.from(next ?? []).map(entry => String(entry)))
                if (!before.includes(id) && (Config.options?.iris?.bubbles?.extras?.[id]?.enable ?? false))
                    updates["iris.bubbles.extras." + id + ".enable"] = false
            Config.setNestedValues(updates)
        }
        else if (/^iris\.bubbles\.extras\.\w+\.(enable|place)$/.test(String(spec.path))) {
            // A bubble switched on to float (or moved off the Island) leaves the Island's list.
            const id = String(spec.path).split(".")[3]
            const updates = ({})
            updates[spec.path] = next
            const place = String(spec.path).endsWith(".place") ? String(next)
                : String(Config.options?.iris?.bubbles?.extras?.[id]?.place ?? IrisPieces.defaultPlace)
            const floating = (spec.path.endsWith(".enable") ? Boolean(next) : true) && place !== "island"
            const listed = Array.from(Config.options?.iris?.bar?.pieces ?? []).map(entry => String(entry))
            if (floating && listed.includes(id)) updates["iris.bar.pieces"] = listed.filter(entry => entry !== id)
            Config.setNestedValues(updates)
        }
        else Config.setNestedValue(spec.path, next)
    }
    // A leading "!" negates either form, so a row can hide on one value of a choice.
    function choicesOf(spec: var): var {
        // A choice row may carry a Ryoku-specific vocabulary (Ink/Wallpaper/Custom
        // for accent and highlight); fall back to the shared list.
        if (IrisStyle.ryokuFrontend && spec.ryokuChoices !== undefined) return spec.ryokuChoices
        if (spec.choicesFrom === "irisModules") return (CustomWidgets.widgets ?? [])
            .filter(widget => String(widget.irisQmlPath ?? "").length > 0)
            .map(widget => ({ label: String(widget.name ?? widget.id), value: "custom:" + widget.id }))
        return spec.choices ?? []
    }
    // A scheme's tuning is listed only when that scheme can be the one in use: the one chosen, or dark and light under Auto.
    function schemeShown(name: string): bool {
        const choice = String(Config.getNestedValue("iris.appearance.scheme", "auto"))
        if (choice !== "auto") return choice === name
        return name !== "ink"
    }
    function shown(spec: var): bool {
        if (spec.showIf && !spec.showIf()) return false
        if (!root.availableIn(spec)) return false
        const when = String(spec.visibleWhen ?? "")
        if (when.length === 0) return true
        const negated = when.startsWith("!")
        const rest = negated ? when.slice(1) : when
        const on = rest.includes("=")
            ? String(Config.getNestedValue(rest.split("=")[0], "")) === rest.split("=")[1]
            : Boolean(Config.getNestedValue(rest, false))
        return negated ? !on : on
    }
    function resettable(spec: var): bool {
        return spec.fallback !== undefined && String(spec.path ?? "").startsWith("iris.")
    }
    // A range reads in its display unit, with as many decimals as its step needs there.
    function rangeText(spec: var, value: var): string {
        if (spec.zeroLabel && Number(value) === 0) return String(spec.zeroLabel)
        const scale = Number(spec.scale ?? 1)
        const shownStep = Math.abs(Number(spec.step ?? 1) * scale)
        const decimals = shownStep > 0 && shownStep < 1 ? Math.min(2, Math.ceil(-Math.log10(shownStep) - 1e-9)) : 0
        // Percent sits on the figure ("40%"), other units keep their space ("12 px").
        return Number(Number(value) * scale).toFixed(decimals).replace(/(\.\d*?)0+$/, "$1").replace(/\.$/, "") + String(spec.unit ?? "").replace(/^ %$/, "%")
    }
    function modified(spec: var): bool {
        return root.resettable(spec) && !root.same(root.currentValue(spec), spec.fallback)
    }

    // Glass widgets wear the shell's glass (frost and edge), so those rows show while widgets are iRiS faces.
    readonly property bool widgetGlass: DesktopWidgetDesign.current === "iris"
    readonly property var pairings: [
        { label: "Shima", value: "iris", values: ["", "", ""] },
        { label: "Pro", value: "pro", values: ["", "", "Inter"] },
        { label: "Rounded", value: "rounded", values: ["Rubik", "Rubik", ""] },
        { label: "Classic", value: "classic", values: ["Noto Sans", "Readex Pro", ""] },
        { label: "Montserrat", value: "montserrat", values: ["Montserrat", "Montserrat", "Montserrat"] },
        { label: "Flex", value: "flex", values: ["Roboto Flex", "Roboto Flex", "Roboto Flex"] },
        { label: "Technical", value: "technical", values: ["Space Grotesk", "Space Grotesk", "JetBrainsMono Nerd Font"] }
    ].filter(pairing => pairing.values.every(face => face.length === 0 || face === IrisStyle.faceText || Qt.fontFamilies().includes(face)))

    // In the Ryoku frontend the family is paper-and-ink, but the Colour tab still
    // drives real colour: the accent and highlight follow the wallpaper (native
    // matugen primary) or a chosen hue, badges pick their source, and the light a
    // body carries is honoured. Material, glass and the wallpaper-tint knobs stay
    // fixed tokens under paper-and-ink, so those are withheld rather than shown
    // dead (a control that cannot work is not offered). Everything else -
    // geometry, motion, layout, size, type scale - drives both frontends.
    readonly property var ryokuGatedPaths: new Set([
        "iris.appearance.theme.surface", "iris.appearance.tint",
        "iris.appearance.glass.mode", "iris.appearance.glass.tint", "iris.appearance.glass.blur",
        "iris.appearance.theme.rimTint", "iris.appearance.theme.lines", "iris.appearance.theme.contrast",
        "iris.appearance.surfaces.cards.light", "iris.appearance.surfaces.controlCenter.light",
        "iris.appearance.surfaces.panels.light", "iris.appearance.surfaces.spotlight.light",
        "iris.appearance.surfaces.gallery.light", "iris.appearance.surfaces.settings.light",
        "iris.appearance.surfaces.menus.light", "iris.appearance.scheme"
    ])
    function availableIn(spec: var): bool {
        if (!IrisStyle.ryokuFrontend) return true
        const path = String(spec.path ?? "")
        // Paper-and-ink carries its own light and dark, so the per-scheme tuning has nothing to act on there.
        return !root.ryokuGatedPaths.has(path) && !path.startsWith("iris.appearance.tune.")
    }

    // The palettes iRiS lists (every one is still in the shared Themes page): what the apps wear, one swatch each.
    // What each scheme calls its materials: the three stored names stay black, graphite and midnight.
    readonly property var materialChoices: {
        const names = IrisStyle.ink ? ["Washi", "Kraft", "Mist"] : IrisStyle.light ? ["Snow", "Silver", "Sky"] : ["Black", "Graphite", "Midnight"]
        return [{ label: names[0], value: "black", get swatch() { return IrisStyle.materialSwatch("black") } },
            { label: names[1], value: "graphite", get swatch() { return IrisStyle.materialSwatch("graphite") } },
            { label: names[2], value: "midnight", get swatch() { return IrisStyle.materialSwatch("midnight") } },
            { label: "Wallpaper", value: "wallpaper", get swatch() { return IrisStyle.materialSwatch("wallpaper") } },
            { label: "Theme", value: "theme", get swatch() { return IrisStyle.materialSwatch("theme") } }]
    }
    function schemeName(name: string): string {
        return Translation.tr(({ dark: "Dark mode", ink: "Ink", light: "Light mode" })[name] ?? "Dark mode")
    }
    // The state of what this row depends on, said where the person is looking, so a row never reads as broken.
    // Where the mode in use comes from, so Auto never reads as a mystery.
    function schemeNote(): string {
        const name = root.schemeName(IrisStyle.scheme)
        const choice = String(Config.getNestedValue("iris.appearance.scheme", "auto"))
        return choice === "auto" ? Translation.tr("In use: %1, from the system").arg(name) : Translation.tr("In use: %1").arg(name)
    }
    function glassNote(): string {
        const mode = String(Config.getNestedValue("iris.appearance.glass.mode", "off"))
        return mode === "off" && IrisStyle.glassy ? Translation.tr("Off here, but Lume frost is on for the %1 scheme, so surfaces are frost anyway.").arg(root.schemeName(IrisStyle.scheme)) : ""
    }
    // Adaptive works by nudging fills, lines, corners, shadows and light a few tens of percent: this says what it is doing now.
    function adaptiveNote(): string {
        if (Number(Config.getNestedValue("iris.appearance.adaptive", 0)) <= 0) return Translation.tr("Off: your values apply as they are.")
        if (!IrisMood.sampled) return Translation.tr("Reading the wallpaper…")
        const pct = name => { const v = Math.round((IrisMood.factor(name) - 1) * 100); return (v < 0 ? "−" : "+") + Math.abs(v) + " %" }
        return Translation.tr("Now: fills %1, lines %2, corners %3, shadows %4, light %5").arg(pct("fill")).arg(pct("lines")).arg(pct("shape")).arg(pct("shadow")).arg(pct("lightReach"))
    }
    readonly property var studioRows: [
        { target: "material", group: "Look", label: "Look", description: "Ryoku's paper-and-ink language, or the original look Shima is based on (iNiR, by snowarch). Colour, material and glass live on the original look only.", path: "iris.appearance.frontend", kind: "choice", fallback: "ryoku", choices: [{label:"Ryoku",value:"ryoku"},{label:"Original",value:"inir"}] },
        { target: "material", group: "Scheme", label: "Scheme", description: "Auto follows the system: dark or light. Ink is a softer mode in between, paper and ink instead of white and black.", path: "iris.appearance.scheme", kind: "choice", fallback: "auto", choices: [{label:"Auto",value:"auto",glyph:"auto_awesome"},{label:"Dark mode",value:"dark",glyph:"dark_mode"},{label:"Ink",value:"ink",glyph:"ink_pen"},{label:"Light mode",value:"light",glyph:"light_mode"}], keywords: ["light", "dark", "ink", "sumi", "washi", "paper", "japanese", "day", "night", "mode", "theme", "bright", "white", "black", "claro", "oscuro", "tinta", "modo"] , note: () => root.schemeNote() },
        { target: "material", group: "Dark look", showIf: () => root.schemeShown("dark"), label: "Tone", description: "Lifts or dims this mode's material.", path: "iris.appearance.tune.dark.tone", kind: "range", fallback: 0, min: -30, max: 30, step: 2, unit: " %", keywords: ["brightness", "dim", "darker", "lighter", "grey", "gray", "eyes", "glare", "tone"] },
        { target: "material", group: "Dark look", showIf: () => root.schemeShown("dark"), label: "Colour strength", description: "How strong accents and colours read in this mode.", path: "iris.appearance.tune.dark.colour", kind: "range", fallback: 100, min: 0, max: 100, step: 5, unit: " %", keywords: ["saturation", "vivid", "muted", "pastel", "strong", "blue", "accent", "colour", "color"] },
        { target: "material", group: "Dark look", showIf: () => root.schemeShown("dark"), label: "Widget colour", description: "How colourful the desktop widgets read in this mode: their accents and the glass over the wallpaper.", path: "iris.appearance.tune.dark.widgets", kind: "range", fallback: 100, min: 40, max: 160, step: 5, unit: " %", keywords: ["widgets", "desktop", "dull", "faded", "muted", "washed", "vivid", "bright", "colourful", "colorful", "saturation", "colour", "color", "dark"] },
        { target: "material", group: "Dark look", showIf: () => root.schemeShown("dark"), label: "Lume frost", description: "Surfaces become frost the wallpaper shows through, as thick as reading needs, instead of solid.", path: "iris.appearance.tune.dark.lume", kind: "switch", fallback: false, keywords: ["dark", "tone", "colour", "color", "lume", "frost", "tune"] },
        { target: "material", group: "Ink look", showIf: () => root.schemeShown("ink"), label: "Tone", description: "Lifts or dims this mode's material.", path: "iris.appearance.tune.ink.tone", kind: "range", fallback: 0, min: -30, max: 30, step: 2, unit: " %", keywords: ["brightness", "dim", "darker", "lighter", "grey", "gray", "eyes", "glare", "tone"] },
        { target: "material", group: "Ink look", showIf: () => root.schemeShown("ink"), label: "Colour strength", description: "How strong accents and colours read in this mode.", path: "iris.appearance.tune.ink.colour", kind: "range", fallback: 100, min: 0, max: 100, step: 5, unit: " %", keywords: ["saturation", "vivid", "muted", "pastel", "strong", "blue", "accent", "colour", "color"] },
        { target: "material", group: "Ink look", showIf: () => root.schemeShown("ink"), label: "Widget colour", description: "How colourful the desktop widgets read in this mode: their accents and the glass over the wallpaper.", path: "iris.appearance.tune.ink.widgets", kind: "range", fallback: 120, min: 40, max: 160, step: 5, unit: " %", keywords: ["widgets", "desktop", "dull", "faded", "muted", "washed", "vivid", "bright", "colourful", "colorful", "saturation", "colour", "color", "ink"] },
        { target: "material", group: "Ink look", showIf: () => root.schemeShown("ink"), label: "Lume frost", description: "Surfaces become frost the wallpaper shows through, as thick as reading needs, instead of solid.", path: "iris.appearance.tune.ink.lume", kind: "switch", fallback: true, keywords: ["ink", "tone", "colour", "color", "lume", "frost", "tune"] },
        { target: "material", group: "Light look", showIf: () => root.schemeShown("light"), label: "Tone", description: "Lifts or dims this mode's material.", path: "iris.appearance.tune.light.tone", kind: "range", fallback: 0, min: -30, max: 30, step: 2, unit: " %", keywords: ["brightness", "dim", "darker", "lighter", "grey", "gray", "eyes", "glare", "tone"] },
        { target: "material", group: "Light look", showIf: () => root.schemeShown("light"), label: "Colour strength", description: "How strong accents and colours read in this mode.", path: "iris.appearance.tune.light.colour", kind: "range", fallback: 85, min: 0, max: 100, step: 5, unit: " %", keywords: ["saturation", "vivid", "muted", "pastel", "strong", "blue", "accent", "colour", "color"] },
        { target: "material", group: "Light look", showIf: () => root.schemeShown("light"), label: "Widget colour", description: "How colourful the desktop widgets read in this mode: their accents and the glass over the wallpaper.", path: "iris.appearance.tune.light.widgets", kind: "range", fallback: 110, min: 40, max: 160, step: 5, unit: " %", keywords: ["widgets", "desktop", "dull", "faded", "muted", "washed", "vivid", "bright", "colourful", "colorful", "saturation", "colour", "color", "light"] },
        { target: "material", group: "Light look", showIf: () => root.schemeShown("light"), label: "Lume frost", description: "Surfaces become frost the wallpaper shows through, as thick as reading needs, instead of solid.", path: "iris.appearance.tune.light.lume", kind: "switch", fallback: true, keywords: ["light", "tone", "colour", "color", "lume", "frost", "tune"] },
        { target: "material", group: "Material", label: "Material", description: "What every surface is made of. Raised steps, fills and the frame follow it.", path: "iris.appearance.theme.surface", kind: "choice", fallback: "black", choices: root.materialChoices },
        { target: "material", group: "Adaptive", label: "Adapt to the wallpaper", description: "Reads the wallpaper's brightness, contrast and colour and shapes Shima from it: calmer images round corners and soften shadows, busy or bright ones sharpen and firm up lines, colourful ones carry more light. 0 keeps your values exactly.", path: "iris.appearance.adaptive", kind: "range", fallback: 0, min: 0, max: 100, step: 5, unit: " %" , note: () => root.adaptiveNote() },
        { target: "material", group: "Material", label: "Fills", description: "Groups, tracks, hovered and pressed controls.", path: "iris.appearance.theme.fill", kind: "range", fallback: 100, min: 30, max: 200, step: 5, unit: " %" },
        { target: "material", group: "Material", label: "Lines", description: "Hairlines and borders. 0 removes them.", path: "iris.appearance.theme.lines", kind: "range", fallback: 100, min: 0, max: 200, step: 5, unit: " %" },
        { target: "material", group: "Material", label: "Shadows", description: "Under bodies floating over windows.", path: "iris.appearance.theme.shadow", kind: "range", fallback: 100, min: 0, max: 160, step: 5, unit: " %" },
        { target: "material", group: "Material", label: "Button rows", description: "Rows of round controls (the Island's pages, the player's transport, the side panels' header) can sit on one plate: Veil darkens what is behind, Glass wears the glass edge, Solid is a reading card. Its corners follow the bubbles' shape.", path: "iris.appearance.controlPlate", kind: "choice", fallback: "none", choices: [{label:"None",value:"none",glyph:"block"},{label:"Veil",value:"veil",glyph:"gradient"},{label:"Glass",value:"glass",glyph:"blur_on"},{label:"Solid",value:"solid",glyph:"square"}], keywords: ["buttons", "controls", "plate", "capsule", "frame", "glass", "veil", "transport", "navigation", "toolbar", "marco"] },
        { target: "material", group: "Glass", label: "Glass", description: "Glass shows the blurred wallpaper behind every Shima surface.", path: "iris.appearance.glass.mode", kind: "choice", fallback: "off", choices: [{label:"Off",value:"off",glyph:"crop_square"},{label:"Glass",value:"wallpaper",glyph:"blur_on"}], note: () => root.glassNote() },
        { target: "material", group: "Glass", label: "Tint", showIf: () => IrisStyle.glassy, description: "How much of the material stays over the glass. Shima raises it on bright wallpapers so text stays legible.", path: "iris.appearance.glass.tint", kind: "range", fallback: 58, min: 12, max: 96, step: 2, unit: " %" },
        { target: "material", group: "Glass", label: "Frost", description: "How blurred the wallpaper is behind the glass.", path: "iris.appearance.glass.blur", showIf: () => IrisStyle.glassWallpaper, kind: "range", fallback: 100, min: 20, max: 100, step: 5, unit: " %" },
        { target: "material", group: "Edges", label: "Style", description: "The edge every surface wears, on every material. Line is an even hairline; Lit catches the light at the top and fades down the sides, like the edge of glass. Glass keeps its lit edge either way.", kind: "choice", bundle: ["iris.appearance.theme.rim", "iris.appearance.theme.edges"], choices: [{label:"None",value:"none",glyph:"crop_free",values:[false,"line"]},{label:"Line",value:"line",glyph:"crop_square",values:[true,"line"]},{label:"Lit",value:"light",glyph:"light_mode",values:[true,"light"]}], keywords: ["edge", "edges", "border", "outline", "rim", "hairline", "light", "glass", "borde", "bordes", "contorno"] },
        { target: "material", group: "Edges", label: "Outline colour", path: "iris.appearance.theme.rimTint", showIf: () => IrisStyle.edgeStyle === "line", kind: "choice", fallback: "neutral", choices: [{label:"Neutral",value:"neutral"},{label:"Accent",value:"accent"},{label:"Highlight",value:"highlight"}] },
        { target: "material", group: "Edges", label: "Outline width", path: "iris.appearance.theme.rimWidth", showIf: () => IrisStyle.edgeStyle === "line", kind: "range", fallback: 1, min: 1, max: 3, unit: " px" },
        { target: "material", group: "Edges", label: "Edge light", description: "How much light the top of each edge catches, like the cut edge of glass. On glass it is what keeps it visible over a dark desktop.", path: "iris.appearance.glass.edgeLight", showIf: () => IrisStyle.edgeStyle === "light" || IrisStyle.glassy || root.widgetGlass, kind: "range", fallback: 34, min: 0, max: 100, step: 2, unit: " %", zeroLabel: "Off", keywords: ["edge", "rim", "highlight", "border", "outline", "shine", "borde", "filo", "brillo"] },
        { target: "material", group: "Edges", label: "Edge line", description: "The same edge where it does not face the light. With Blur it also smooths the stepped curves.", path: "iris.appearance.glass.edgeLine", showIf: () => IrisStyle.edgeStyle === "light" || IrisStyle.glassy || root.widgetGlass, kind: "range", fallback: 10, min: 0, max: 60, step: 2, unit: " %", zeroLabel: "Off", keywords: ["edge", "border", "outline", "jagged", "aliasing", "borde", "serrucho"] },
        { target: "material", group: "Edges", label: "Edge width", path: "iris.appearance.glass.edgeWidth", showIf: () => IrisStyle.edgeStyle === "light" || IrisStyle.glassy || root.widgetGlass, kind: "range", fallback: 1.5, min: 0.5, max: 3, step: 0.5, unit: " px", keywords: ["edge", "border", "thickness", "borde", "grosor"] },
        { target: "material", group: "Edges", label: "Edge colour", description: "Scene takes the wallpaper's light.", path: "iris.appearance.glass.edgeColour", showIf: () => IrisStyle.edgeStyle === "light" || IrisStyle.glassy || root.widgetGlass, kind: "choice", fallback: "scene", choices: [{label:"Scene",value:"scene",glyph:"wallpaper"},{label:"White",value:"white",glyph:"light_mode"},{label:"Accent",value:"accent",glyph:"palette"},{label:"Highlight",value:"highlight",glyph:"auto_awesome"}], keywords: ["edge", "colour", "color", "borde", "color del borde"] },
        { target: "material", group: "Shape", label: "Base corners", description: "Corners of side panels, buttons and text fields. Panels with their own corners keep them.", path: "iris.appearance.expandedRadius", kind: "range", fallback: 28, min: 16, max: 40, unit: " px", keywords: ["expanded", "radius", "buttons", "fields", "panels", "round"] },
        { target: "material", group: "Shape", label: "Corners", description: "Every radius in the family; nested corners keep stepping down.", path: "iris.appearance.theme.shape", kind: "range", fallback: 100, min: 30, max: 160, step: 5, unit: " %" },
        { target: "material", group: "Shape", label: "One shape for all", mirror: true, description: "The Island, the Dock and the bubbles share the bubbles' shape.", path: "iris.appearance.theme.linkShapes", kind: "switch", fallback: false, keywords: ["same", "match", "link", "linked", "together", "sync", "unify", "everything", "one shape", "mismo", "igual", "todo", "juntos", "vincular"] },
        { target: "material", group: "Shape", label: "All shapes", mirror: true, visibleWhen: "iris.appearance.theme.linkShapes", path: "iris.appearance.theme.pieceShape", kind: "choice", fallback: "circle", choices: [{label:"Circle",value:"circle",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "material", group: "Shape", label: "Bubble shape", mirror: true, visibleWhen: "!iris.appearance.theme.linkShapes", path: "iris.appearance.theme.pieceShape", kind: "choice", fallback: "circle", choices: [{label:"Circle",value:"circle",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "capsule", "pill", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "material", group: "Shape", label: "Island shape", mirror: true, visibleWhen: "!iris.appearance.theme.linkShapes", path: "iris.bar.shape", kind: "choice", fallback: "auto", choices: [{label:"Auto",value:"auto",glyph:"auto_awesome"},{label:"Round",value:"round",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "capsule", "pill", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "material", group: "Shape", label: "Dock shape", mirror: true, visibleWhen: "!iris.appearance.theme.linkShapes", path: "iris.dock.shape", kind: "choice", fallback: "auto", choices: [{label:"Auto",value:"auto",glyph:"auto_awesome"},{label:"Round",value:"round",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "capsule", "pill", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "material", group: "Shape", label: "Fusion", description: "How deeply shapes melt where they meet. 0 keeps crisp joins.", path: "iris.appearance.theme.melt", kind: "range", fallback: 0, min: 0, max: 200, step: 5, unit: " %" },
        { target: "material", group: "Shape", label: "Spacing", description: "Padding, gaps and control sizes.", path: "iris.appearance.density", kind: "range", fallback: 1, min: 0.8, max: 1.35, step: 0.05, unit: "×" },
        { target: "material", group: "Icons", label: "Icon pack", description: "How every Shima plate is drawn. The app-icon look is a plain plate with the icon in a gradient of its own colour.", path: "iris.appearance.icons.style", kind: "choice", fallback: "iris", choices: [{label:"Shima",value:"iris",glyph:"apps"},{label:"Tiles",value:"tile",glyph:"gradient"}], keywords: ["icons", "glyphs", "symbols", "material symbols", "pack", "tile", "tiles", "plate", "gradient", "depth", "app icon", "iconos", "glifos", "paquete", "degradado", "placa"] },
        { target: "material", group: "Icons", label: "Plate", description: "The plate behind each icon. Black is the app-icon look; White or Surface read better on bright screens.", path: "iris.appearance.icons.plate", kind: "choice", fallback: "black", visibleWhen: "iris.appearance.icons.style=tile", choices: [{label:"Black",value:"black",get swatch() { return IrisStyle.plateBlackBase }},{label:"White",value:"white",get swatch() { return IrisStyle.plateWhiteBase }},{label:"Tint",value:"tint",get swatch() { return IrisStyle.identity.lavender }},{label:"Surface",value:"surface",get swatch() { return IrisStyle.surfaceHighestOpaque }}], keywords: ["plate", "background", "behind", "black", "white", "light", "dark", "tile", "icon", "placa", "fondo", "negro", "blanco", "claro", "oscuro"] },
        { target: "material", group: "Frame", label: "Close the shell around the screen", description: "One black band on every edge, with the screen's corners turned inward. Windows and the Island sit inside it.", path: "iris.surround.enable", kind: "switch", fallback: true },
        { target: "material", group: "Frame", label: "Frame width", path: "iris.surround.thickness", visibleWhen: "iris.surround.enable", kind: "range", fallback: 10, min: 4, max: 28, unit: " px" },
        { target: "material", group: "Frame", label: "Screen corners", description: "How round the frame turns inward at the corners of the screen.", path: "iris.surround.radius", visibleWhen: "iris.surround.enable", kind: "range", fallback: 22, min: 0, max: 44, unit: " px" },
        { section: "frameMusic", group: "On the edges", label: "Music on the edges", description: "Let music swell the Shima frame itself. The frame is enabled under Appearance.", path: "background.edgeWidgets.organic.enable", kind: "switch", fallback: false, keywords: ["music", "frame", "visualizer", "cava"] },
        { section: "frameMusic", group: "Frame response", label: "Edges", description: "Which sides of the frame move with the music.", path: "iris.surround.musicEdges", visibleWhen: "background.edgeWidgets.organic.enable", showIf: () => Boolean(Config.options?.iris?.surround?.enable), kind: "choice", fallback: "sides", choices: [{label:"Left and right",value:"sides"},{label:"Top and bottom",value:"horizontal"},{label:"All four",value:"all"}] },
        { section: "frameMusic", group: "Frame response", label: "Strength", description: "How far the frame swells into the screen. The default is more visible than before.", path: "iris.surround.musicStrength", visibleWhen: "background.edgeWidgets.organic.enable", showIf: () => Boolean(Config.options?.iris?.surround?.enable), kind: "range", fallback: 160, min: 50, max: 300, step: 10, unit: " %" },
        { section: "frameMusic", group: "Frame response", label: "Sensitivity", description: "How strongly quiet passages move the frame.", path: "iris.surround.musicSensitivity", visibleWhen: "background.edgeWidgets.organic.enable", showIf: () => Boolean(Config.options?.iris?.surround?.enable), kind: "range", fallback: 140, min: 50, max: 250, step: 10, unit: " %" },
        { section: "frameMusic", group: "Frame response", label: "Travel speed", description: "How quickly the swell flows along the edge.", path: "iris.surround.musicSpeed", visibleWhen: "background.edgeWidgets.organic.enable", showIf: () => Boolean(Config.options?.iris?.surround?.enable), kind: "range", fallback: 100, min: 40, max: 200, step: 10, unit: " %" },
        { section: "frameMusic", group: "Finish", label: "Appearance", description: "Sculpted is pure shape. Etched catches a fine line on the inner edge. Satin lets light fall softly across the band.", path: "iris.surround.musicAppearance", visibleWhen: "background.edgeWidgets.organic.enable", showIf: () => Boolean(Config.options?.iris?.surround?.enable), kind: "choice", fallback: "etched", choices: [{label:"Sculpted",value:"sculpted"},{label:"Etched",value:"etched"},{label:"Satin",value:"satin"}] },
        { section: "frameMusic", group: "Finish", label: "Light colour", description: "Pearl follows Shima ink; the other choices follow your theme and wallpaper.", path: "iris.surround.musicColour", visibleWhen: "background.edgeWidgets.organic.enable", showIf: () => Boolean(Config.options?.iris?.surround?.enable) && String(Config.options?.iris?.surround?.musicAppearance ?? "etched") !== "sculpted", kind: "choice", fallback: "pearl", choices: [{label:"Pearl",value:"pearl",get swatch() { return IrisStyle.text }},{label:"Accent",value:"accent",get swatch() { return IrisStyle.accent }},{label:"Highlight",value:"highlight",get swatch() { return IrisStyle.secondaryAccent }},{label:"Wallpaper",value:"wallpaper",get swatch() { return IrisStyle.wallpaperLight }}] },
        { section: "frameMusic", group: "Finish", label: "Light", description: "How much of the chosen colour rests on the moving frame.", path: "iris.surround.musicLight", visibleWhen: "background.edgeWidgets.organic.enable", showIf: () => Boolean(Config.options?.iris?.surround?.enable) && String(Config.options?.iris?.surround?.musicAppearance ?? "etched") !== "sculpted", kind: "range", fallback: 35, min: 0, max: 100, step: 5, unit: " %", zeroLabel: "Off" },
        { section: "frameMusic", group: "Finish", label: "Light width", description: "A hairline for Etched, or a wider falloff for Satin.", path: "iris.surround.musicLightWidth", visibleWhen: "background.edgeWidgets.organic.enable", showIf: () => Boolean(Config.options?.iris?.surround?.enable) && String(Config.options?.iris?.surround?.musicAppearance ?? "etched") !== "sculpted", kind: "range", fallback: 5, min: 1, max: 18, unit: " px" },
        { target: "colour", group: "Accent", label: "System accent", description: "Selection and controls across Shima. Activity colours keep their identity.", path: "iris.appearance.accent", kind: "choice", fallback: "blue", choices: [{label:"Blue",value:"blue",get swatch() { return IrisStyle.accents.blue }},{label:"Mint",value:"mint",get swatch() { return IrisStyle.accents.mint }},{label:"Rose",value:"rose",get swatch() { return IrisStyle.accents.rose }},{label:"Lilac",value:"lilac",get swatch() { return IrisStyle.accents.lilac }},{label:"Theme",value:"theme",get swatch() { return IrisStyle.themeAccent }},{label:"Wallpaper",value:"wallpaper"},{label:"Custom",value:"custom"}], ryokuChoices: [{label:"Ink",value:"ink",get swatch() { return IrisStyle.text }},{label:"Wallpaper",value:"wallpaper"},{label:"Custom",value:"custom"}] },
        { target: "colour", group: "Accent", label: "Accent hue", visibleWhen: "iris.appearance.accent=custom", path: "iris.appearance.theme.accentHue", kind: "hue", fallback: 212 },
        { target: "colour", group: "Highlight", label: "Highlight", description: "The glanced detail: timers, badges and activity rings.", path: "iris.appearance.highlight", kind: "choice", fallback: "orange", choices: [{label:"Orange",value:"orange",get swatch() { return IrisStyle.highlights.orange }},{label:"Yellow",value:"yellow",get swatch() { return IrisStyle.highlights.yellow }},{label:"Red",value:"red",get swatch() { return IrisStyle.highlights.red }},{label:"Pink",value:"pink",get swatch() { return IrisStyle.highlights.pink }},{label:"Green",value:"green",get swatch() { return IrisStyle.highlights.green }},{label:"Accent",value:"accent"},{label:"Theme",value:"theme",get swatch() { return IrisStyle.vividHighlight(Appearance.colors.colTertiary, IrisStyle.highlights.orange) }},{label:"Wallpaper",value:"wallpaper"},{label:"Custom",value:"custom"}], ryokuChoices: [{label:"Ink",value:"ink",get swatch() { return IrisStyle.text }},{label:"Wallpaper",value:"wallpaper"},{label:"Custom",value:"custom"}] },
        { target: "colour", group: "Highlight", label: "Highlight hue", visibleWhen: "iris.appearance.highlight=custom", path: "iris.appearance.theme.highlightHue", kind: "hue", fallback: 32 },
        { target: "colour", group: "Colour layer", section: "anime", label: "Anime colour layer", description: "Anime-inspired accents without changing your layout; switch off to restore your colours.", path: "iris.appearance.anime.enabled", kind: "switch", fallback: false, keywords: ["anime", "weeb", "otaku", "japanese", "japones", "palette", "accent", "colour", "color"] },
        { target: "colour", group: "Colour layer", section: "anime", label: "Palette", visibleWhen: "iris.appearance.anime.enabled", path: "iris.appearance.anime.palette", kind: "choice", fallback: "sakura",
            choices: Object.keys(IrisStyle.animePalettes).map(key => ({ label: IrisStyle.animePalettes[key].label, value: key, get swatch() { return IrisStyle.animeAccent(IrisStyle.accents.blue, key, 1) } })),
            keywords: ["anime", "weeb", "otaku", "palette", "colour", "color", "pink", "rose", "cyan", "magenta", "violet", "purple", "green", "gold", "amber"].concat(Object.values(IrisStyle.animePalettes).map(entry => entry.label.toLowerCase())) },
        { target: "colour", group: "Colour layer", section: "anime", label: "Strength", visibleWhen: "iris.appearance.anime.enabled", description: "How far the palette pulls the accents away from your own; 0 keeps them exactly.", path: "iris.appearance.anime.strength", kind: "range", fallback: 60, min: 0, max: 100, step: 5, unit: " %", keywords: ["anime", "weeb", "otaku", "palette", "intensidad", "intensity", "strength", "blend"] },
        { target: "colour", group: "Colour layer", section: "anime", label: "Recolour the highlight", visibleWhen: "iris.appearance.anime.enabled", description: "The highlight joins the palette. Off keeps your own highlight, even when it is set to Accent.", path: "iris.appearance.anime.highlight", kind: "switch", fallback: false, keywords: ["anime", "weeb", "otaku", "palette", "highlight", "colour", "color"] },
        { target: "colour", group: "Airing", section: "anime", label: "Airing bubble", description: "Shows the next anime episode on the Island. Its card lists what is coming and follows the shows you tap.", path: "iris.bubbles.extras.anime.enable", kind: "switch", fallback: false, keywords: ["anime", "weeb", "otaku", "airing", "japanese", "japones", "tracker", "following", "episodes", "episodio", "schedule", "live tv"] },
        { target: "colour", group: "Airing", section: "anime", label: "Episodes in the card", description: "How many upcoming episodes the Airing bubble lists.", path: "iris.anime.shows", kind: "range", fallback: 5, min: 3, max: 8, keywords: ["anime", "weeb", "otaku", "airing", "episodes", "episodio", "following", "shows"] },
        { target: "colour", group: "Badges", label: "Unread counts", description: "The family's red, your accent, the highlight, or a quiet neutral.", path: "iris.appearance.theme.badge", kind: "choice", fallback: "alert", choices: [{label:"Alert",value:"alert",get swatch() { return IrisStyle.identity.red }},{label:"Accent",value:"accent"},{label:"Highlight",value:"highlight",get swatch() { return IrisStyle.secondaryAccent }},{label:"Neutral",value:"neutral",get swatch() { return IrisStyle.surfaceHighestOpaque }}] },
        { target: "colour", group: "Light", label: "Light", description: "What an open card or panel is lit by: its own colour — the sky for weather, the highlight for timers — or the wallpaper's. It pours in from where the body grew.", path: "iris.appearance.aura", kind: "choice", fallback: "subtle", choices: [{label:"Off",value:"off",glyph:"light_off"},{label:"Subtle",value:"subtle",glyph:"light_mode"},{label:"Vivid",value:"vivid",glyph:"wb_sunny"}] },
        { target: "colour", group: "Light", label: "Reach", description: "How far the light goes into a body: just its head, or deeper.", path: "iris.appearance.theme.lightReach", kind: "range", fallback: 100, min: 50, max: 300, step: 5, unit: " %" },
        { target: "colour", group: "Light", label: "Glow", description: "Bodies that float over your windows (cards, Spotlight, banners, the Dock menu) cast a halo in the accent colour instead of a dark shadow.", path: "iris.appearance.theme.glow", kind: "range", fallback: 0, min: 0, max: 100, step: 5, unit: " %" },
        { target: "colour", group: "Wallpaper", label: "Wallpaper tint", description: "How much of the wallpaper the black carries. The Island stays black; raised surfaces and fills take the trace.", path: "iris.appearance.tint", kind: "range", fallback: 0, min: 0, max: 100, step: 5, unit: "%" },
        { target: "type", group: "Text", label: "Size", description: "Text across Shima, on top of the system scale.", path: "iris.appearance.theme.text", kind: "range", fallback: 100, min: 85, max: 125, step: 5, unit: " %" },
        { target: "type", group: "Text", label: "Weight", description: "Every weight in Shima one step lighter or firmer.", path: "iris.appearance.theme.weight", kind: "choice", fallback: "regular", choices: [{label:"Light",value:"light"},{label:"Regular",value:"regular"},{label:"Bold",value:"bold"}], keywords: ["font", "weight", "bold", "thin", "peso", "negrita"] },
        { target: "type", group: "Text", label: "Contrast", description: "Secondary and quiet text.", path: "iris.appearance.theme.contrast", kind: "range", fallback: 100, min: 60, max: 150, step: 5, unit: " %" },
        { target: "type", group: "Faces", label: "Pairing", description: "Sets the three faces below together.", kind: "choice", previewFont: true, bundle: ["iris.appearance.fontFamily", "iris.appearance.titleFontFamily", "iris.appearance.numbersFontFamily"], choices: root.pairings, keywords: ["font", "fonts", "typeface", "typography", "fuente", "tipografia", "letra"] },
        { target: "type", group: "Faces", label: "Text", description: "Labels, rows and everything you read.", path: "iris.appearance.fontFamily", kind: "choice", installedFonts: true, previewFont: true, fallback: "", choices: [{label:"Inter",value:""},{label:"Noto Sans",value:"Noto Sans"},{label:"Montserrat",value:"Montserrat"},{label:"Rubik",value:"Rubik"},{label:"Readex Pro",value:"Readex Pro"},{label:"Roboto Flex",value:"Roboto Flex"},{label:"Gabarito",value:"Gabarito"},{label:"Space Grotesk",value:"Space Grotesk"},{label:"Oxanium",value:"Oxanium"},{label:"JetBrains Mono",value:"JetBrainsMono Nerd Font"}], keywords: ["font", "fonts", "typeface", "typography", "fuente", "tipografia", "letra"] },
        { target: "type", group: "Faces", label: "Titles", description: "Page titles and card heads.", path: "iris.appearance.titleFontFamily", kind: "choice", installedFonts: true, previewFont: true, fallback: "", choices: [{label:"Inter Display",value:""},{label:"Noto Sans",value:"Noto Sans"},{label:"Montserrat",value:"Montserrat"},{label:"Rubik",value:"Rubik"},{label:"Readex Pro",value:"Readex Pro"},{label:"Roboto Flex",value:"Roboto Flex"},{label:"Gabarito",value:"Gabarito"},{label:"Space Grotesk",value:"Space Grotesk"},{label:"Oxanium",value:"Oxanium"},{label:"JetBrains Mono",value:"JetBrainsMono Nerd Font"}], keywords: ["font", "fonts", "typeface", "typography", "fuente", "tipografia", "letra"] },
        { target: "type", group: "Faces", label: "Figures", description: "Clocks, timers and levels.", path: "iris.appearance.numbersFontFamily", kind: "choice", installedFonts: true, previewFont: true, fallback: "", choices: [{label:"Rubik",value:""},{label:"Inter",value:"Inter"},{label:"Noto Sans",value:"Noto Sans"},{label:"Montserrat",value:"Montserrat"},{label:"Readex Pro",value:"Readex Pro"},{label:"Roboto Flex",value:"Roboto Flex"},{label:"Gabarito",value:"Gabarito"},{label:"Space Grotesk",value:"Space Grotesk"},{label:"Oxanium",value:"Oxanium"},{label:"JetBrains Mono",value:"JetBrainsMono Nerd Font"}], keywords: ["font", "fonts", "typeface", "typography", "fuente", "tipografia", "letra"] },
        { target: "type", group: "Faces", label: "Figure weight", path: "iris.appearance.figureWeight", kind: "choice", fallback: "bold", choices: [{label:"Light",value:"light"},{label:"Regular",value:"regular"},{label:"Bold",value:"bold"}] },
        { target: "motion", group: "Motion", label: "Movement", description: "How every shape Shima grows moves. Direct is the Shima motion: fast out, settles without bouncing. Liquid and Elastic are springs with a settle or a bounce, Glide is slow and even, Snap is terse.", path: "iris.appearance.morph", kind: "choice", fallback: "direct", choices: [{label:"Direct",value:"direct",glyph:"north_east"},{label:"Liquid",value:"liquid",glyph:"water_drop"},{label:"Glide",value:"glide",glyph:"swipe_right_alt"},{label:"Snap",value:"snap",glyph:"bolt"},{label:"Elastic",value:"elastic",glyph:"airwave"}] },
        { target: "motion", group: "Curve", label: "Curve", visibleWhen: "iris.appearance.motion", description: "The path a Direct shape follows as it opens or adjusts; closing is always absorbed back into its origin. Expressive leaves fast and lands softly; Standard is even; Gentle eases in and out; Swift is short and sharp.", path: "iris.appearance.theme.curve", kind: "choice", fallback: "expressive", choices: [{label:"Expressive",value:"expressive",glyph:"north_east"},{label:"Standard",value:"standard",glyph:"trending_up"},{label:"Gentle",value:"gentle",glyph:"moving"},{label:"Swift",value:"swift",glyph:"bolt"},{label:"Custom",value:"custom",glyph:"draw"}] },
        { target: "motion", group: "Curve", label: "Shape", visibleWhen: "iris.appearance.motion", description: "Drag the two handles. The curve becomes Custom.", path: "iris.appearance.theme.curvePoints", kind: "curve", fallback: [0.16, 1, 0.3, 1] },
        { target: "motion", group: "Timing", label: "Open and close", visibleWhen: "iris.appearance.motion", description: "How long a shape takes to grow out of what opened it and fold back.", path: "iris.appearance.theme.openTime", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Timing", label: "Adjusting", visibleWhen: "iris.appearance.motion", description: "How long an open shape takes to change size, like switching pages.", path: "iris.appearance.theme.moveTime", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Timing", label: "Content arrives", visibleWhen: "iris.appearance.motion", description: "Lower shows what is inside while the shape is still growing; higher waits until it has formed.", path: "iris.appearance.theme.contentTiming", kind: "range", fallback: 100, min: 30, max: 170, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Island", visibleWhen: "iris.appearance.motion", description: "Speed of this surface's morph. 100 % follows the timing above.", path: "iris.appearance.surfaces.island.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Cards", visibleWhen: "iris.appearance.motion", path: "iris.appearance.surfaces.cards.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Control Center", visibleWhen: "iris.appearance.motion", path: "iris.appearance.surfaces.controlCenter.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Side panels", visibleWhen: "iris.appearance.motion", path: "iris.appearance.surfaces.panels.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Spotlight", visibleWhen: "iris.appearance.motion", path: "iris.appearance.surfaces.spotlight.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Settings", visibleWhen: "iris.appearance.motion", path: "iris.appearance.surfaces.settings.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Menus", visibleWhen: "iris.appearance.motion", path: "iris.appearance.surfaces.menus.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Wallpaper gallery", visibleWhen: "iris.appearance.motion", path: "iris.appearance.surfaces.gallery.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %" },
        { target: "motion", group: "Per surface", label: "Volume and song pill", visibleWhen: "iris.appearance.motion", path: "iris.appearance.surfaces.osd.speed", kind: "range", fallback: 100, min: 40, max: 250, step: 5, unit: " %", keywords: ["osd", "hud", "volume"] },
        { target: "motion", group: "Touch", label: "Press depth", description: "How far buttons, bubbles and the Island dip under a press. 0 keeps them still.", path: "iris.appearance.theme.press", kind: "range", fallback: 100, min: 0, max: 200, step: 10, unit: " %" },
        { target: "motion", group: "Motion", label: "Duration", visibleWhen: "iris.appearance.motion", description: "How long a shape takes to grow out of what opened it.", path: "iris.appearance.motionDuration", kind: "range", fallback:220,min:100,max:400,step:10,unit:" ms" },
        { target: "motion", group: "Motion", label: "Bounce", visibleWhen: "iris.appearance.motion", description: "How far arrivals and moves pass their place before settling, as a share of the style's own. 0 never bounces; leaving never does.", path: "iris.appearance.theme.bounce", kind: "range", fallback: 100, min: 0, max: 200, step: 10, unit: " %" },
        { target: "island", group: "Layout", label: "Island layout", description: "Hug its content in the middle, hug one end, span the whole edge as a bar, or run a thin menu bar with the Island hanging from it as a notch.", path: "iris.bar.layout", kind: "choice", fallback: "island", choices: [{label:"Island",value:"island",glyph:"pill"},{label:"Left",value:"left",glyph:"align_horizontal_left"},{label:"Right",value:"right",glyph:"align_horizontal_right"},{label:"Full width",value:"full",glyph:"width_full"},{label:"Menu bar",value:"menubar",glyph:"toolbar"}], keywords: ["menu bar", "notch", "macbook", "mac", "strip", "bar"] },
        { target: "island", group: "Layout", label: "Screen edge", path: "iris.bar.position", kind: "choice", fallback: "top", choices: [{label:"Top",value:"top",glyph:"vertical_align_top"},{label:"Bottom",value:"bottom",glyph:"vertical_align_bottom"},{label:"Left",value:"left",glyph:"align_horizontal_left"},{label:"Right",value:"right",glyph:"align_horizontal_right"}] },
        { target: "island", group: "Layout", label: "Height", description: "The resting Island, and how deep its notch melts into the edge.", path: "iris.bar.height", kind: "range", fallback: 42, min: 32, max: 64, unit: " px" },
        { target: "island", group: "Layout", label: "Gap from the edge", path: "iris.bar.margin", kind: "range", fallback:8,min:0,max:24,unit:" px" },
        { target: "island", group: "Layout", label: "Menu bar strip", visibleWhen: "iris.bar.layout=menubar", description: "Transparent leaves your items on the wallpaper, like macOS; Band lays a bar under them.", path: "iris.bar.strip", kind: "choice", fallback: "clear", choices: [{label:"Transparent",value:"clear",glyph:"blur_off"},{label:"Band",value:"band",glyph:"toolbar"}], keywords: ["transparent", "clear", "macos", "tahoe", "menu bar", "strip", "band", "background"] },
        { target: "island", group: "Shape", label: "One shape for all", description: "The Island, the Dock and the bubbles share the bubbles' shape.", path: "iris.appearance.theme.linkShapes", kind: "switch", fallback: false, keywords: ["same", "match", "link", "linked", "together", "sync", "unify", "everything", "one shape", "mismo", "igual", "todo", "juntos", "vincular"] },
        { target: "island", group: "Shape", label: "Island shape", visibleWhen: "!iris.appearance.theme.linkShapes", description: "Auto keeps the Shima capsule. With Square, lower the Notch curve to straighten the shoulders.", path: "iris.bar.shape", kind: "choice", fallback: "auto", choices: [{label:"Auto",value:"auto",glyph:"auto_awesome"},{label:"Round",value:"round",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "capsule", "pill", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "island", group: "Shape", label: "All shapes", mirror: true, visibleWhen: "iris.appearance.theme.linkShapes", path: "iris.appearance.theme.pieceShape", kind: "choice", fallback: "circle", choices: [{label:"Circle",value:"circle",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "island", group: "Notch", label: "Notch", description: "The Island melts into the screen edge, like the notch it grows from. Off, it floats clear of the edge.", path: "iris.bar.notch", kind: "switch", fallback: true, keywords: ["notch", "attach", "melt", "edge", "dynamic island", "float", "floating", "muesca"] },
        { target: "island", group: "Shape", label: "Notch curve", visibleWhen: "iris.bar.notch", description: "How wide the shoulders are where the Island turns into its edge.", path: "iris.bar.notchCurve", kind: "range", fallback: 100, min: 20, max: 200, step: 5, unit: " %" },
        { target: "island", group: "Shape", label: "Open corners", description: "Auto follows the Island's shape; a size here overrides it.", path: "iris.appearance.surfaces.island.radius", kind: "range", fallback: 0, min: 0, max: 44, unit: " px", zeroLabel: "Auto", keywords: ["corners", "radius", "round", "expanded", "open"] },
        { target: "island", group: "At rest", label: "Clock", description: "What the resting Island shows beside the time.", path: "iris.bar.clockStyle", kind: "choice", fallback: "dateTime", choices: [{label:"Time",value:"time"},{label:"Date",value:"dateTime"},{label:"Weather",value:"weather"}] },
        { target: "island", group: "At rest", label: "Utility island", description: "Emerges beside the Island; the tray hides when no apps are present.", path: "iris.bar.auxiliary", kind: "choice", fallback: "tray", choices: [{label:"Tray",value:"tray"},{label:"Timers",value:"tools"},{label:"Sound",value:"sound"},{label:"Microphone",value:"mic"},{label:"None",value:"none"}] },
        { target: "island", group: "At rest", label: "Clock size", description: "The time and date on the resting Island. The Island grows to fit them.", path: "iris.bar.clockScale", kind: "range", fallback: 100, min: 80, max: 150, step: 5, unit: " %" },
        { target: "island", group: "At rest", label: "Clock accent", description: "The colour of the time's separator and the day number on the resting Island.", path: "iris.bar.clockAccent", kind: "choice", fallback: "highlight", choices: [{label:"Highlight",value:"highlight",get swatch() { return IrisStyle.secondaryAccent }},{label:"Accent",value:"accent",get swatch() { return IrisStyle.accent }},{label:"Plain",value:"plain",get swatch() { return IrisStyle.text }}] },
        { target: "island", group: "At rest", label: "Breathing room", description: "Space around what the resting Island shows.", path: "iris.bar.padding", kind: "range", fallback: 100, min: 50, max: 250, step: 10, unit: " %" },
        { target: "island", group: "At rest", label: "Bubble gap", description: "How far the bubbles beside the Island rest from it.", path: "iris.bar.satelliteGap", kind: "range", fallback: 6, min: 0, max: 24, unit: " px" },
        { target: "island", group: "At rest", label: "Bubble size", description: "The bubbles beside the Island, as a share of its height.", path: "iris.bar.satelliteScale", kind: "range", fallback: 100, min: 70, max: 100, step: 5, unit: " %" },
        { target: "island", group: "Desktop page", label: "Header", description: "What sits behind the time when the Island shows your desktop.", path: "iris.bar.desktopBanner", kind: "choice", fallback: "wallpaper", choices: [{label:"Wallpaper",value:"wallpaper"},{label:"None",value:"none"}] },
        { target: "island", group: "Desktop page", label: "Header fade", description: "How far the wallpaper melts into the Island under the time. Lower keeps more of it showing.", path: "iris.bar.desktopBannerFade", visibleWhen: "iris.bar.desktopBanner=wallpaper", kind: "range", fallback: 100, min: 0, max: 100, step: 5, unit: " %", keywords: ["header", "banner", "gradient", "fade", "transparency", "wallpaper", "hero", "degradado"] },
        { target: "island", group: "Desktop page", label: "Header top", description: "How the wallpaper melts into the Island at the top, under its buttons. Lower lets it rise higher, up to where the Island meets the edge.", path: "iris.bar.desktopBannerTop", visibleWhen: "iris.bar.desktopBanner=wallpaper", kind: "range", fallback: 100, min: 0, max: 100, step: 5, unit: " %", keywords: ["header", "banner", "top", "gradient", "fade", "transparency", "wallpaper", "hero", "degradado"] },
        { target: "island", group: "Desktop page", label: "Header veil", description: "How much the wallpaper is dimmed behind the time. Below 100 % the time can be harder to read on bright wallpapers.", path: "iris.bar.desktopBannerVeil", visibleWhen: "iris.bar.desktopBanner=wallpaper", kind: "range", fallback: 100, min: 0, max: 100, step: 5, unit: " %", keywords: ["header", "banner", "dim", "dark", "veil", "scrim", "contrast", "wallpaper"] },
        { target: "island", group: "Desktop page", label: "Header blur", description: "Softens the wallpaper behind the time.", path: "iris.bar.desktopBannerBlur", visibleWhen: "iris.bar.desktopBanner=wallpaper", kind: "range", fallback: 0, min: 0, max: 100, step: 5, unit: " %", keywords: ["header", "banner", "blur", "soft", "frost", "wallpaper"] },
        { target: "island", group: "Desktop page", label: "Blocks", description: "What sits under the time, in the order you switch them on. You can also arrange them on the Island itself: open its desktop page and tap the pencil.", path: "iris.bar.desktopBlocks", kind: "pieces", fallback: ["profile", "context", "forecast", "agenda", "modules"], choices: [{label:"Profile",value:"profile"},{label:"Current app",value:"context"},{label:"Forecast",value:"forecast"},{label:"Up next",value:"agenda"},{label:"Vitals",value:"vitals"},{label:"Modules",value:"modules"}] },
        { target: "island", group: "Desktop page", label: "Your modules", description: "Modules written with the Shima SDK sit in the page's Modules block. Make one with inir customWidgets create <name>; it shows up here.", path: "iris.bar.rightModules", kind: "pieces", fallback: [], choicesFrom: "irisModules", showIf: () => (CustomWidgets.widgets ?? []).some(widget => String(widget.irisQmlPath ?? "").length > 0), keywords: ["sdk", "custom", "module", "plugin", "extension", "widget"] },
        { target: "island", group: "Desktop page", label: "Block style", description: "Plain rows sit on the Island's black like the rest of the page; Grouped puts the forecast and vitals strips on quiet plates.", path: "iris.bar.blockStyle", kind: "choice", fallback: "plain", choices: [{label:"Plain",value:"plain",glyph:"view_agenda"},{label:"Grouped",value:"grouped",glyph:"splitscreen"}] },
        { target: "island", group: "Pages", label: "Page width", description: "How wide the open Island is. The activity page keeps its proportion.", path: "iris.bar.pageWidth", kind: "range", fallback: 440, min: 360, max: 600, step: 10, unit: " px" },
        { target: "island", group: "Pages", label: "Navigation", description: "What the open Island's navigation row offers, in the order you switch them on. Pages come first; scroll over the row, or swipe sideways anywhere on the open Island, to move between them.", path: "iris.bar.navItems", kind: "pieces", fallback: ["media", "activity", "desktop", "tray", "tools", "focus", "today", "controls", "settings"], choices: [{label:"Now playing",value:"media"},{label:"Live activities",value:"activity"},{label:"Desktop",value:"desktop"},{label:"Tray",value:"tray"},{label:"Timers",value:"tools"},{label:"Focus panel",value:"focus"},{label:"Today panel",value:"today"},{label:"Quick controls",value:"controls"},{label:"Settings",value:"settings"}] },
        { target: "island", group: "Pages", label: "Page buttons", description: "What holds the navigation row. Auto follows Appearance › Button rows; a plate follows the bubbles' shape, with the buttons inside it concentric.", path: "iris.bar.navFrame", kind: "choice", fallback: "auto", choices: [{label:"Auto",value:"auto",glyph:"auto_awesome"},{label:"None",value:"none",glyph:"block"},{label:"Veil",value:"veil",glyph:"gradient"},{label:"Glass",value:"glass",glyph:"blur_on"},{label:"Solid",value:"solid",glyph:"square"}], keywords: ["navigation", "buttons", "tabs", "frame", "plate", "capsule", "glass", "blur", "background", "marco"] },
        { target: "island", group: "Player page", label: "Blocks", description: "What the player page shows, in the order you switch them on.", path: "iris.bar.mediaBlocks", kind: "pieces", fallback: ["player", "timeline", "transport", "players", "levels"], choices: [{label:"Now playing",value:"player"},{label:"Timeline",value:"timeline"},{label:"Controls",value:"transport"},{label:"Other players",value:"players"},{label:"App volume",value:"levels"}] },
        { target: "pieces", group: "Size", label: "Bubble size", description: "Bubbles off the Island, as a share of the Island's height.", path: "iris.bubbles.scale", kind: "range", fallback: 100, min: 60, max: 140, step: 5, unit: " %" },
        { target: "pieces", group: "Size", label: "One shape for all", mirror: true, description: "The Island, the Dock and the bubbles share the bubbles' shape.", path: "iris.appearance.theme.linkShapes", kind: "switch", fallback: false, keywords: ["same", "match", "link", "linked", "together", "sync", "unify", "everything", "one shape", "mismo", "igual", "todo", "juntos", "vincular"] },
        { target: "pieces", group: "Size", label: "Bubble shape", visibleWhen: "!iris.appearance.theme.linkShapes", description: "The profile of every bubble, bar and satellite. Squircles and squares read like app tiles.", keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "pill", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"], path: "iris.appearance.theme.pieceShape", kind: "choice", fallback: "circle", choices: [{label:"Circle",value:"circle",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}] },
        { target: "pieces", group: "Size", label: "All shapes", mirror: true, visibleWhen: "iris.appearance.theme.linkShapes", path: "iris.appearance.theme.pieceShape", kind: "choice", fallback: "circle", choices: [{label:"Circle",value:"circle",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "pieces", group: "On the contour", label: "Group into a bar", description: "Bubbles resting in the same place share one plate and read as a small bar; off, each one floats as its own disc.", path: "iris.bubbles.cluster", kind: "switch", fallback: true },
        { target: "pieces", group: "On the contour", label: "Keep them on the frame", description: "Bubbles in a corner or on an edge sit on the shell's own edge, where they read as a swelling of it, instead of floating over your windows.", path: "iris.bubbles.attach", kind: "switch", fallback: true },
        { target: "pieces", group: "On the contour", label: "Meet the edge", description: "Notch: they melt into the edge with the Island's curved shoulders. Weld: they touch it. Gap: they keep the Island's margin from it.", path: "iris.bubbles.join", visibleWhen: "iris.bubbles.attach", kind: "choice", fallback: "notch", choices: [{label:"Notch",value:"notch"},{label:"Weld",value:"weld"},{label:"Gap",value:"gap"}] },
        { target: "pieces", group: "On the contour", label: "Shoulders", description: "How deep the curve is where a bubble melts into the edge.", path: "iris.bubbles.notchCurve", visibleWhen: "iris.bubbles.join=notch", kind: "range", fallback: 100, min: 30, max: 200, step: 5, unit: " %" },
        { target: "pieces", group: "Opening bodies", label: "Join to opener", description: "When settled, let a card or Control Center become one silhouette with the bubble or Island that opened it.", path: "iris.appearance.surfaces.cards.joinOrigin", kind: "switch", fallback: false },
        { target: "bodies", group: "Cards", label: "Design", description: "A whole look for every card at once. Anything you set below still wins.", path: "iris.appearance.surfaces.cards.design", kind: "choice", fallback: "welded", choices: [{label:"Welded",value:"welded",glyph:"join_inner"},{label:"Floating",value:"floating",glyph:"filter_none"},{label:"Plain",value:"plain",glyph:"crop_square"},{label:"Vibrant",value:"vibrant",glyph:"auto_awesome"}] },
        { target: "bodies", group: "Cards", label: "Room around the content", description: "Auto follows the design.", path: "iris.appearance.surfaces.cards.pad", kind: "range", fallback: 0, min: 0, max: 160, step: 5, unit: " %", zeroLabel: "Auto" },
        { target: "bodies", group: "Cards", label: "Handle", description: "A small bar on the edge facing its bubble or the Island. Tap it to put the card away.", path: "iris.appearance.surfaces.cards.grabber", kind: "choice", fallback: "auto", choices: [{label:"Auto",value:"auto"},{label:"Show",value:"on"},{label:"Hide",value:"off"}] },
        { target: "bodies", group: "Cards", label: "Corners", description: "Auto keeps the family's shape for cards.", path: "iris.appearance.surfaces.cards.radius", kind: "range", fallback: 0, min: 0, max: 44, unit: " px", zeroLabel: "Auto" },
        { target: "bodies", group: "Joining", label: "Opens", description: "Where a card or the Control Center grows from a bubble: away from the edge it sits on, or along it.", path: "iris.appearance.theme.placement", kind: "choice", fallback: "auto", choices: [{label:"Away from the edge",value:"auto",glyph:"open_in_new"},{label:"Along the edge",value:"along",glyph:"swap_vert"}] },
        { target: "bodies", group: "Joining", label: "Air", description: "The space between what opened and the bar or bubble it came from.", path: "iris.appearance.theme.air", kind: "range", fallback: 8, min: 0, max: 40, unit: " px" },
        { target: "bodies", group: "Joining", label: "Fusion", description: "How deeply bodies melt where they touch. 0 keeps joins crisp; the join to a bubble and the notch keep their own shape.", path: "iris.appearance.theme.melt", kind: "range", fallback: 0, min: 0, max: 200, step: 5, unit: " %" },
        { target: "bodies", group: "Cards", label: "Width", description: "Auto keeps the family's width.", path: "iris.appearance.surfaces.cards.width", kind: "range", fallback: 0, min: 0, max: 560, step: 10, unit: " px", zeroLabel: "Auto" },
        { target: "bodies", group: "Cards", label: "Stand off its piece", description: "How far a card rests from the bubble or Island that opened it. 0 welds them together, which is the Shima default.", path: "iris.appearance.surfaces.cards.gap", kind: "range", fallback: 0, min: 0, max: 40, unit: " px", zeroLabel: "Welded" },
        { target: "bodies", group: "Cards", label: "Light", description: "Its own colour, the wallpaper's, or plain black.", path: "iris.appearance.surfaces.cards.light", kind: "choice", fallback: "inherit", choices: [{label:"Own",value:"inherit",glyph:"auto_awesome"},{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Off",value:"off",glyph:"light_off"}] },
        { target: "bodies", group: "Card contents", label: "Header", description: "The glyph, name and figure at the top of the sound and microphone cards.", path: "iris.appearance.surfaces.cards.header", kind: "switch", fallback: true },
        { target: "bodies", group: "Card contents", label: "Devices", description: "Pick the output or input right inside the card.", path: "iris.appearance.surfaces.cards.devices", kind: "switch", fallback: true },
        { target: "bodies", group: "Card contents", label: "App volumes", description: "Each app's level under the sound card.", path: "iris.appearance.surfaces.cards.mixer", kind: "switch", fallback: true },
        { target: "bodies", group: "Control Center", label: "Corners", description: "Auto keeps the family's shape for the panel.", path: "iris.appearance.surfaces.controlCenter.radius", kind: "range", fallback: 0, min: 0, max: 44, unit: " px", zeroLabel: "Auto" },
        { target: "bodies", group: "Control Center", label: "Light", description: "Its own colour, the wallpaper's, or plain black.", path: "iris.appearance.surfaces.controlCenter.light", kind: "choice", fallback: "inherit", choices: [{label:"Own",value:"inherit",glyph:"auto_awesome"},{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Off",value:"off",glyph:"light_off"}] },
        { target: "bodies", group: "Control Center", label: "What it carries", description: "Every control the panel can hold, in the order you switch them on. Arrange it in place to resize or reorder them.", path: "iris.controlCenter.modules", kind: "pieces", fallback: ["platter", "media", "darkMode", "nightLight", "levels", "idle", "snip", "devices", "record", "notifications"], choices: IrisControlOptions.catalogue.map(entry => ({ label: entry.label, value: entry.id })) },
        { target: "bodies", group: "Control Center", label: "Columns", description: "How many controls fit across. More columns make everything smaller.", path: "iris.controlCenter.columns", kind: "range", fallback: 4, min: 3, max: 6, keywords: ["grid", "layout"] },
        { target: "bodies", group: "Control Center", label: "Names under the controls", description: "Off shows a name in the header while you point at it.", path: "iris.controlCenter.labels", kind: "switch", fallback: false },
        { target: "bodies", group: "Control Center", label: "Control shape", description: "Round draws each switch as a disc with its name under it; Tiles are rounded squares.", path: "iris.controlCenter.controls", kind: "choice", fallback: "tiles", choices: [{label:"Tiles",value:"tiles",glyph:"grid_view"},{label:"Round",value:"round",glyph:"radio_button_checked"}] },
        { target: "bodies", group: "Control Center", label: "Accent", description: "What a switch lights up with. Colourful gives each one its own hue; Mono lights them white.", path: "iris.controlCenter.accent", kind: "choice", fallback: "system", choices: [{label:"Shima",value:"system"},{label:"Accent",value:"accent"},{label:"Colourful",value:"colourful"},{label:"Mono",value:"mono"}], keywords: ["colour", "color", "tint", "toggle"] },
        { target: "bodies", group: "Control Center", label: "Slider fill", path: "iris.controlCenter.sliders", kind: "choice", fallback: "neutral", choices: [{label:"Neutral",value:"neutral"},{label:"Accent",value:"accent"}], keywords: ["brightness", "volume", "level"] },
        { target: "bodies", group: "Control Center", label: "Connections plate", description: "The four round switches on the Connections plate.", path: "iris.controlCenter.platter", kind: "pieces", fallback: ["network", "bluetooth", "focus", "gameMode"], choices: [{label:"Network",value:"network"},{label:"Bluetooth",value:"bluetooth"},{label:"VPN",value:"vpn"},{label:"Hotspot",value:"hotspot"},{label:"WARP",value:"warp"},{label:"Focus",value:"focus"},{label:"Game",value:"gameMode"},{label:"Profile",value:"profiles"},{label:"Awake",value:"idle"},{label:"Dark",value:"darkMode"},{label:"Night light",value:"nightLight"},{label:"Output",value:"audio"},{label:"Mic",value:"mic"}], keywords: ["wifi", "bluetooth", "platter"] },
        { target: "bodies", group: "Control Center", label: "Levels", description: "Which sliders stand side by side in Levels.", path: "iris.controlCenter.levels", kind: "pieces", fallback: ["brightness", "volume", "microphone"], choices: [{label:"Brightness",value:"brightness"},{label:"Volume",value:"volume"},{label:"Microphone",value:"microphone"}], keywords: ["slider", "brightness", "volume", "mic"] },
        { target: "bodies", group: "Control Center", label: "Opens as", description: "The Island itself becomes the Control Center, or a panel hangs from it. From a bubble it always grows beside that bubble.", path: "iris.controlCenter.opens", kind: "choice", fallback: "island", choices: [{label:"The Island",value:"island",glyph:"pill"},{label:"A panel",value:"panel",glyph:"web_asset"}] },
        { target: "bodies", group: "Control Center", label: "Width", path: "iris.controlCenter.width", kind: "range", fallback:360,min:320,max:540,step:10,unit:" px" },
        { target: "bodies", group: "Visualizer", label: "Visualizer", description: "How what plays is drawn on the resting Island, its player page and the Visualizer bubble.", path: "iris.player.visualizer.style", kind: "choice", fallback: "capsules",
            choices: [{label:"Capsules",value:"capsules",glyph:"graphic_eq"},{label:"Equalizer",value:"rise",glyph:"equalizer"},{label:"Dots",value:"dots",glyph:"more_horiz"},{label:"Wave",value:"wave",glyph:"airwave"},{label:"Ring",value:"ring",glyph:"brightness_empty"}],
            keywords: ["visualizer", "visualiser", "cava", "spectrum", "equalizer", "ecualizador", "bars", "barras", "wave", "onda", "dots", "puntos", "ring", "anillo"] },
        { target: "bodies", group: "Visualizer", label: "Bands", description: "How many parts of the sound it follows.", path: "iris.player.visualizer.bars", kind: "range", fallback: 5, min: 3, max: 9, step: 1 },
        { target: "bodies", group: "Visualizer", label: "Visualizer colour", description: "The artwork's own colour, the accent, the highlight or the ink.", path: "iris.player.visualizer.colour", kind: "choice", fallback: "art",
            choices: [{label:"Artwork",value:"art"},{label:"Accent",value:"accent"},{label:"Highlight",value:"highlight"},{label:"Ink",value:"ink"}] },
        { target: "bodies", group: "Player", label: "Round album cover", path: "iris.player.roundCover", kind: "switch", fallback: false },
        { target: "bodies", group: "Player", label: "Blurred album background", description: "Tints the expanded Island and media cards with the cover.", path: "iris.player.artworkBackground", kind: "switch", fallback:true },
        { target: "places", group: "Panel look", label: "Corners", description: "Auto keeps the family's shape for side panels.", path: "iris.appearance.surfaces.panels.radius", kind: "range", fallback: 0, min: 0, max: 44, unit: " px", zeroLabel: "Auto" },
        { target: "places", group: "Panel look", label: "Light", description: "Its own colour, the wallpaper's, or plain black.", path: "iris.appearance.surfaces.panels.light", kind: "choice", fallback: "inherit", choices: [{label:"Own",value:"inherit",glyph:"auto_awesome"},{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Off",value:"off",glyph:"light_off"}] },
        { target: "places", group: "Spotlight", label: "Corners", description: "Auto keeps the family's shape for Spotlight.", path: "iris.appearance.surfaces.spotlight.radius", kind: "range", fallback: 0, min: 0, max: 44, unit: " px", zeroLabel: "Auto" },
        { target: "places", group: "Spotlight", label: "Light", description: "Its own colour, the wallpaper's, or plain black.", path: "iris.appearance.surfaces.spotlight.light", kind: "choice", fallback: "inherit", choices: [{label:"Own",value:"inherit",glyph:"auto_awesome"},{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Off",value:"off",glyph:"light_off"}] },
        { target: "places", group: "Spotlight", label: "Opens as", description: "Floating settles in place as a sheet of its own and the Island keeps its face; The Island makes the Island itself become Spotlight, joined to its edge, with the search field on the Island's side.", path: "iris.palette.opens", kind: "choice", fallback: "floating", choices: [{label:"Floating",value:"floating",glyph:"web_asset"},{label:"The Island",value:"island",glyph:"pill"}] },
        { target: "places", group: "Spotlight", label: "Width", path: "iris.palette.width", kind: "range", fallback:640,min:420,max:900,step:10,unit:" px" },
        { target: "places", group: "Wallpaper gallery", label: "Corners", description: "Auto keeps the family's shape for the gallery.", path: "iris.appearance.surfaces.gallery.radius", kind: "range", fallback: 0, min: 0, max: 44, unit: " px", zeroLabel: "Auto" },
        { target: "places", group: "Wallpaper gallery", label: "Light", description: "Its own colour, the wallpaper's, or plain black.", path: "iris.appearance.surfaces.gallery.light", kind: "choice", fallback: "inherit", choices: [{label:"Own",value:"inherit",glyph:"auto_awesome"},{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Off",value:"off",glyph:"light_off"}] },
        { target: "places", group: "Wallpaper gallery", label: "Layout", description: "Showcase puts the chosen wallpaper on a large stage above one row; Strip is two compact rows; Wall fills three rows to browse many at once.", path: "iris.wallpaper.layout", kind: "choice", fallback: "showcase", choices: [{label:"Strip",value:"strip",glyph:"view_carousel"},{label:"Showcase",value:"showcase",glyph:"featured_video"},{label:"Wall",value:"wall",glyph:"view_module"}] },
        { target: "places", group: "Wallpaper gallery", label: "Gallery width", path: "iris.wallpaper.width", kind: "range", fallback:960,min:640,max:1400,step:40,unit:" px" },
        { target: "places", group: "Wallpaper gallery", label: "Preview size", path: "iris.wallpaper.thumbnailSize", kind: "range", fallback:228,min:160,max:320,step:8,unit:" px" },
        { target: "places", group: "Settings", label: "Corners", description: "Auto keeps the family's shape for Settings.", path: "iris.appearance.surfaces.settings.radius", kind: "range", fallback: 0, min: 0, max: 44, unit: " px", zeroLabel: "Auto" },
        { target: "places", group: "Settings", label: "Light", description: "Its own colour, the wallpaper's, or plain black.", path: "iris.appearance.surfaces.settings.light", kind: "choice", fallback: "inherit", choices: [{label:"Own",value:"inherit",glyph:"auto_awesome"},{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Off",value:"off",glyph:"light_off"}] },
        { target: "places", group: "Menus", label: "Size", description: "Compact rows fit more in less; Regular keeps them roomy.", path: "iris.appearance.surfaces.menus.density", kind: "choice", fallback: "compact", choices: [{label:"Compact",value:"compact",glyph:"density_small"},{label:"Regular",value:"regular",glyph:"density_medium"}], keywords: ["menu", "context menu", "right click", "tray", "size", "density", "small"] },
        { target: "places", group: "Menus", label: "Corners", description: "Auto keeps the family's shape for menus.", path: "iris.appearance.surfaces.menus.radius", kind: "range", fallback: 0, min: 0, max: 44, unit: " px", zeroLabel: "Auto" },
        { target: "places", group: "Menus", label: "Light", description: "Its own colour, the wallpaper's, or plain black.", path: "iris.appearance.surfaces.menus.light", kind: "choice", fallback: "inherit", choices: [{label:"Own",value:"inherit",glyph:"auto_awesome"},{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Off",value:"off",glyph:"light_off"}] },
        { target: "transients", group: "Banners", label: "Width", path: "iris.notifications.width", visibleWhen: "iris.modules.notificationPopup", kind: "range", fallback: 380, min: 340, max: 560, step: 10, unit: " px" },
        { target: "transients", group: "Feedback", label: "Level style", description: "Capsule fills the whole pill like a slider; Bar keeps a slim line; Minimal is a ring around the icon. The Island uses it too.", path: "iris.osd.style", visibleWhen: "iris.modules.osd", kind: "choice", fallback: "capsule", choices: [{label:"Capsule",value:"capsule",glyph:"toggle_on"},{label:"Bar",value:"bar",glyph:"linear_scale"},{label:"Minimal",value:"minimal",glyph:"progress_activity"}], keywords: ["osd", "hud", "volume", "brightness", "slider", "look", "apple"] },
        { target: "transients", group: "Feedback", label: "Show the percentage", path: "iris.osd.figure", visibleWhen: "iris.modules.osd", kind: "switch", fallback: true, keywords: ["osd", "number", "value", "percent"] },
        { target: "transients", group: "Feedback", label: "Pill corners", description: "Auto keeps it a capsule.", path: "iris.appearance.surfaces.osd.radius", visibleWhen: "iris.modules.osd", kind: "range", fallback: 0, min: 0, max: 32, unit: " px", zeroLabel: "Auto", keywords: ["osd", "radius", "shape"] },
        { target: "transients", group: "Feedback", label: "Pill light", description: "Own lights volume indigo, the mic orange and brightness in the highlight.", path: "iris.appearance.surfaces.osd.light", visibleWhen: "iris.modules.osd", kind: "choice", fallback: "inherit", choices: [{label:"Own",value:"inherit",glyph:"auto_awesome"},{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Off",value:"off",glyph:"light_off"}], keywords: ["osd", "colour", "color", "glow", "tint"] },
        { target: "transients", group: "Feedback", label: "OSD width", path: "iris.osd.width", visibleWhen: "iris.modules.osd", kind: "range", fallback: 320, min: 260, max: 520, step: 10, unit: " px" },
        { target: "transients", group: "Feedback", label: "OSD position", description: "Where the level pill shows when the Island is not there to answer, like over a fullscreen game.", path: "iris.osd.position", visibleWhen: "iris.modules.osd", kind: "choice", fallback: "bottom", choices: [{label:"Bottom",value:"bottom",glyph:"vertical_align_bottom"},{label:"Top",value:"top",glyph:"vertical_align_top"}], keywords: ["osd", "volume", "pill", "edge", "place"] },
        { target: "transients", group: "Feedback", label: "Distance from the edge", path: "iris.osd.offset", visibleWhen: "iris.modules.osd", kind: "range", fallback: 18, min: 0, max: 240, step: 2, unit: " px", keywords: ["osd", "margin", "offset", "gap"] },
        { target: "transients", group: "Feedback", label: "Time on screen", description: "Songs and warnings stay a little longer so they can be read.", path: "iris.osd.duration", visibleWhen: "iris.modules.osd", kind: "range", fallback: 1500, min: 800, max: 5000, step: 100, unit: " ms", keywords: ["osd", "timeout", "duration", "hide"] },
        { target: "dock", group: "Look", label: "Screen edge", description: "Auto keeps the Dock on the edge opposite the Island. Choosing the Island's edge swaps them: the Island takes the Dock's place.", path: "iris.dock.position", kind: "choice", fallback: "auto", choices: root.dockEdgeChoices },
        { target: "dock", group: "Look", label: "One shape for all", mirror: true, description: "The Island, the Dock and the bubbles share the bubbles' shape.", path: "iris.appearance.theme.linkShapes", kind: "switch", fallback: false, keywords: ["same", "match", "link", "linked", "together", "sync", "unify", "everything", "one shape", "mismo", "igual", "todo", "juntos", "vincular"] },
        { target: "dock", group: "Look", label: "All shapes", mirror: true, visibleWhen: "iris.appearance.theme.linkShapes", path: "iris.appearance.theme.pieceShape", kind: "choice", fallback: "circle", choices: [{label:"Circle",value:"circle",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "dock", group: "Look", label: "Dock shape", visibleWhen: "!iris.appearance.theme.linkShapes", description: "Auto keeps the Shima capsule. It works on a floating Dock and on a notched one.", path: "iris.dock.shape", kind: "choice", fallback: "auto", choices: [{label:"Auto",value:"auto",glyph:"auto_awesome"},{label:"Round",value:"round",glyph:"circle"},{label:"Squircle",value:"squircle",glyph:"rounded_corner"},{label:"Square",value:"square",glyph:"square"}], keywords: ["square", "squared", "boxy", "rectangle", "sharp", "round", "rounded", "capsule", "pill", "circle", "squircle", "corners", "radius", "curves", "cuadrado", "cuadrada", "redondo", "redonda", "esquinas", "curvas", "forma"] },
        { target: "dock", group: "Notch", label: "Dock notch", description: "The Dock melts into its screen edge like the Island's notch. Off, it floats clear of the edge.", path: "iris.dock.notch", kind: "switch", fallback: true, keywords: ["notch", "attach", "melt", "edge", "float", "floating", "muesca"] },
        { target: "dock", group: "Look", label: "Material", description: "Follow Shima, stay solid, or glass over the blurred wallpaper.", path: "iris.dock.material", kind: "choice", fallback: "inherit", choices: [{label:"Shima",value:"inherit"},{label:"Solid",value:"solid"},{label:"Glass",value:"glass"}] },
        { target: "dock", group: "Icons", label: "Icon size", path: "iris.dock.iconSize", kind: "range", fallback:40,min:28,max:64,unit:" px" },
        { target: "dock", group: "Icons", label: "Magnify on hover", path: "iris.dock.magnification", kind: "switch", fallback:false },
        { target: "dock", group: "Icons", label: "Magnification", visibleWhen: "iris.dock.magnification", description: "How large the icon under the pointer grows; its neighbours follow.", path: "iris.dock.magnifySize", kind: "range", fallback: 150, min: 110, max: 200, step: 5, unit: " %" },
        { target: "dock", group: "Icons", label: "Applications button", description: "Opens Spotlight from the start of the Dock.", path: "iris.dock.launcher", kind: "switch", fallback:true },
        { target: "desktop", group: "Widgets", label: "On the desktop", description: "Tap a widget to add it to this screen or take it away. While arranging, drop one widget on another to stack them. Size, look and position are set on the widget itself: select it while arranging.", path: "background.widgets", kind: "widgets", keywords: ["stack", "stacks", "pages", "group", "carousel", "rotate", "smart stack", "layer", "combine"] },
        { target: "desktop", group: "Widgets", label: "Design", quickName: "Widget design", description: "One look on every widget. Shima draws its own faces, Material keeps each widget's own style, iNstrument reads like a set of gauges and Readout gives figures room.", path: "iris.widgets.design", widgetDesign: true, kind: "choice", fallback: "iris", choices: [{label:"Shima",value:"iris",glyph:"auto_awesome"},{label:"Material",value:"material",glyph:"widgets"},{label:"iNstrument",value:"instrument",glyph:"avg_pace"},{label:"Readout",value:"readout",glyph:"view_agenda"}], keywords: ["global", "instrument", "readout", "shared", "material", "widgets", "style", "look", "all widgets", "match", "tech", "gauges"] },
        { target: "desktop", group: "Widgets", label: "Widgets with a look of their own", description: "Some widgets were given a different design in their Look controls. Match puts them on the design above.", kind: "action", button: "Match", run: () => DesktopWidgetDesign.apply(DesktopWidgetDesign.current), showIf: () => DesktopWidgetDesign.exceptionCount > 0, keywords: ["match", "same", "coherent", "exceptions", "design"] },
        { target: "desktop", group: "Widgets", label: "Undo the last design change", description: "Brings back the design and each widget's own look from before you started changing it.", kind: "action", button: "Undo", run: () => DesktopWidgetDesign.undo(), showIf: () => DesktopWidgetDesign.canUndo, keywords: ["undo", "revert", "restore", "back", "design"] },
        { target: "desktop", group: "Widgets", label: "Widget corners", description: "Individual widget overrides take priority.", path: "iris.widgets.radius", showIf: () => DesktopWidgetDesign.current === "iris", kind: "range", fallback:22,min:0,max:40,unit:" px" },
        { target: "desktop", group: "Widgets", label: "Accents", quickName: "Widget accents", description: "The three colours every widget draws its rings, bars and figures with. Wallpaper takes the generated colours, Shima your accent and highlight, Spectrum fixed blue, orange and teal, Mono one hue in three tones.", path: "iris.widgets.tint", kind: "choice", fallback: "wallpaper", choices: [{label:"Wallpaper",value:"wallpaper",glyph:"wallpaper"},{label:"Shima",value:"system",glyph:"auto_awesome"},{label:"Spectrum",value:"spectrum",glyph:"palette"},{label:"Mono",value:"mono",glyph:"contrast"}], keywords: ["colour", "color", "accent", "palette", "tint", "hue", "widgets"] },
        { target: "desktop", group: "Widgets", label: "Accent strength", description: "Low keeps soft pastels; high makes accents saturated and bright. They stay readable over the wallpaper either way.", path: "iris.widgets.vibrance", kind: "range", fallback: 85, min: 0, max: 100, step: 5, unit: " %", keywords: ["vibrance", "saturation", "strong", "vivid", "accent", "colour", "color"] },
        { target: "desktop", group: "Widgets", label: "Material", description: "Glass frosts the wallpaper behind each widget; Transparent leaves it bare, with nothing drawn behind the widget itself. A widget can choose its own in its Look controls.", path: "iris.widgets.material", showIf: () => DesktopWidgetDesign.current === "iris", kind: "choice", fallback: "glass", choices: [{label:"Glass",value:"glass",glyph:"blur_on"},{label:"Transparent",value:"clear",glyph:"select"},{label:"Solid",value:"solid",glyph:"square"},{label:"Tinted",value:"tinted",glyph:"format_color_fill"}] },
        { target: "desktop", group: "Widgets", label: "Lume on every widget", description: "Every widget reads as if the wallpaper under it were bright and busy: a deeper veil on glass and a backing plate under Transparent, even where it is not needed.", path: "iris.widgets.legibleAlways", kind: "switch", fallback: false, keywords: ["lume", "legibility", "readable", "contrast", "veil", "shadow", "always", "force", "text"] },
        { target: "desktop", group: "Widgets", label: "On bright wallpapers", description: "Where the wallpaper under a widget is light, it turns to frost with dark ink and deeper accents. Off keeps light ink and darkens the glass instead.", path: "iris.widgets.brightWallpapers", kind: "switch", fallback: false, keywords: ["bright", "white", "light", "legible", "readable", "contrast", "ink", "black", "dark text", "frost", "adaptive"] },
        { target: "desktop", group: "Widgets", label: "Titles and figures", path: "iris.widgets.weight", showIf: () => DesktopWidgetDesign.current === "iris", kind: "choice", fallback: "regular", choices: [{label:"Light",value:"light"},{label:"Regular",value:"regular"},{label:"Bold",value:"bold"}] },
        { target: "desktop", group: "Widgets", label: "Surface opacity", path: "iris.widgets.opacity", showIf: () => DesktopWidgetDesign.current === "iris", description: "How much material a widget carries. At 100 % Glass is as thick as its text needs; lower lets more wallpaper through. Lume on every widget keeps it readable.", kind: "range", fallback:100,min:20,max:100,step:5,unit:" %" },
        { target: "desktop", group: "Widgets", label: "Outline", description: "The line around each widget. Auto follows the shell's outline on Glass, Solid and Tinted and leaves Transparent bare. While arranging it always shows.", path: "iris.widgets.outline", showIf: () => DesktopWidgetDesign.current === "iris", kind: "choice", fallback: "auto", choices: [{label:"Auto",value:"auto",glyph:"auto_awesome"},{label:"Always",value:"always",glyph:"crop_square"},{label:"None",value:"none",glyph:"block"}], keywords: ["outline", "border", "frame", "rim", "hairline", "edge", "line", "stroke", "marco", "borde"] },
        { target: "desktop", group: "Widgets", label: "Widgets with their own material", description: "They chose a material or surface opacity in their Look controls, so the rows above don't reach them. Match puts them on these.", kind: "action", button: "Match", run: () => DesktopWidgetDesign.matchSurfaces(), showIf: () => DesktopWidgetDesign.current === "iris" && DesktopWidgetDesign.ownSurfaceCount > 0, keywords: ["match", "same", "material", "glass", "opacity", "own", "reset", "every widget"] },
        { target: "desktop", group: "Desktop menu", label: "Wallpaper at the top", description: "The right-click menu opens on your wallpaper: tap it for the gallery, or the round button for the next one.", path: "iris.desktopMenu.wallpaper", kind: "switch", fallback: true, keywords: ["right click", "context menu", "desktop menu", "wallpaper", "shuffle"] },
        { target: "desktop", group: "Desktop menu", label: "Actions", description: "What the right-click menu offers, in the order you switch them on. They group themselves: the desktop, tools, then the shell.", path: "iris.desktopMenu.items", kind: "pieces", fallback: ["widgets", "customize", "screenshot", "terminal", "settings", "restart"], choices: IrisDesktopActions.catalogue.map(entry => ({ label: entry.label, value: entry.id })), keywords: ["right click", "context menu", "desktop menu", "terminal", "screenshot", "files", "record", "colour picker", "restart"] },
        { target: "motion", group: "Style per surface", label: "Island", description: "A surface can move in a style of its own. Family follows Movement above; None shows and hides it at once.", path: "iris.appearance.surfaces.island.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "motion", group: "Style per surface", label: "Cards", path: "iris.appearance.surfaces.cards.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "motion", group: "Style per surface", label: "Control Center", path: "iris.appearance.surfaces.controlCenter.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "motion", group: "Style per surface", label: "Side panels", path: "iris.appearance.surfaces.panels.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "motion", group: "Style per surface", label: "Spotlight", path: "iris.appearance.surfaces.spotlight.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "motion", group: "Style per surface", label: "Settings", path: "iris.appearance.surfaces.settings.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "motion", group: "Style per surface", label: "Menus", path: "iris.appearance.surfaces.menus.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "motion", group: "Style per surface", label: "Wallpaper gallery", path: "iris.appearance.surfaces.gallery.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "motion", group: "Style per surface", label: "Volume and song pill", path: "iris.appearance.surfaces.osd.morph", visibleWhen: "iris.appearance.motion", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Direct",value:"direct"},{label:"Liquid",value:"liquid"},{label:"Glide",value:"glide"},{label:"Snap",value:"snap"},{label:"Elastic",value:"elastic"},{label:"None",value:"instant"}], keywords: ["morph", "animation", "spring", "bounce", "instant", "no animation", "none"] },
        { target: "material", group: "Material per surface", label: "Cards", description: "Glass or solid for this surface alone. Family follows Glass above.", path: "iris.appearance.surfaces.cards.material", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Solid",value:"solid",glyph:"crop_square"},{label:"Glass",value:"glass",glyph:"blur_on"}], keywords: ["glass", "solid", "blur", "material", "vidrio"] },
        { target: "material", group: "Material per surface", label: "Control Center", path: "iris.appearance.surfaces.controlCenter.material", visibleWhen: "iris.controlCenter.opens=panel", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Solid",value:"solid",glyph:"crop_square"},{label:"Glass",value:"glass",glyph:"blur_on"}], keywords: ["glass", "solid", "blur", "material", "vidrio"] },
        { target: "material", group: "Material per surface", label: "Side panels", path: "iris.appearance.surfaces.panels.material", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Solid",value:"solid",glyph:"crop_square"},{label:"Glass",value:"glass",glyph:"blur_on"}], keywords: ["glass", "solid", "blur", "material", "vidrio"] },
        { target: "material", group: "Material per surface", label: "Spotlight", path: "iris.appearance.surfaces.spotlight.material", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Solid",value:"solid",glyph:"crop_square"},{label:"Glass",value:"glass",glyph:"blur_on"}], keywords: ["glass", "solid", "blur", "material", "vidrio"] },
        { target: "material", group: "Material per surface", label: "Settings", path: "iris.appearance.surfaces.settings.material", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Solid",value:"solid",glyph:"crop_square"},{label:"Glass",value:"glass",glyph:"blur_on"}], keywords: ["glass", "solid", "blur", "material", "vidrio"] },
        { target: "material", group: "Material per surface", label: "Menus", path: "iris.appearance.surfaces.menus.material", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Solid",value:"solid",glyph:"crop_square"},{label:"Glass",value:"glass",glyph:"blur_on"}], keywords: ["glass", "solid", "blur", "material", "vidrio"] },
        { target: "material", group: "Material per surface", label: "Wallpaper gallery", path: "iris.appearance.surfaces.gallery.material", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Solid",value:"solid",glyph:"crop_square"},{label:"Glass",value:"glass",glyph:"blur_on"}], keywords: ["glass", "solid", "blur", "material", "vidrio"] },
        { target: "material", group: "Material per surface", label: "Volume and song pill", path: "iris.appearance.surfaces.osd.material", kind: "choice", fallback: "", choices: [{label:"Family",value:""},{label:"Solid",value:"solid",glyph:"crop_square"},{label:"Glass",value:"glass",glyph:"blur_on"}], keywords: ["glass", "solid", "blur", "material", "vidrio"] }
    ]
    // Every surface's corners in one place, beside the family's: each row is the surface's own (same path, same range),
    // listed here as well because "how round is it" is asked of the look, not of a surface.
    readonly property var cornerLabels: ({ island: "Island", cards: "Cards", controlCenter: "Control Center", panels: "Side panels", spotlight: "Spotlight",
        settings: "Settings", gallery: "Wallpaper gallery", menus: "Menus", osd: "Volume and song pill" })
    readonly property var cornerMirrors: {
        const out = []
        for (const spec of root.studioRows) {
            const found = /^iris\.appearance\.surfaces\.(\w+)\.radius$/.exec(String(spec.path ?? ""))
            if (!found || !root.cornerLabels[found[1]]) continue
            const mirror = Object.assign({}, spec, { target: "material", group: "Corners per surface", label: root.cornerLabels[found[1]], mirror: true,
                keywords: ["corners", "radius", "round", "rounded", "square"] })
            delete mirror.description
            out.push(mirror)
        }
        return out
    }
    // Glass widgets wear the shell's glass edge; Desktop › Widgets shows the same rows beside Outline.
    readonly property var edgeKeys: ["edgeLight", "edgeLine", "edgeWidth", "edgeColour"]
    readonly property var edgeMirrors: {
        const out = []
        for (const spec of root.studioRows) {
            const found = /^iris\.appearance\.glass\.(\w+)$/.exec(String(spec.path ?? ""))
            if (!found || !root.edgeKeys.includes(found[1])) continue
            out.push(Object.assign({}, spec, { target: "desktop", group: "Widgets", mirror: true,
                description: "The edge of glass widgets, the same as the Island's and the Dock's glass.",
                showIf: () => root.widgetGlass && String(Config.options?.iris?.widgets?.outline ?? "auto") !== "none",
                keywords: ["outline", "border", "frame", "edge", "line", "light", "rim", "marco", "borde", "widgets"] }))
        }
        return out
    }
    readonly property var studio: {
        const rows = root.studioRows.slice()
        const at = rows.map(spec => spec.target === "material" && spec.group === "Shape").lastIndexOf(true)
        rows.splice(at < 0 ? rows.length : at + 1, 0, ...root.cornerMirrors)
        const outline = rows.findIndex(spec => spec.path === "iris.widgets.outline")
        rows.splice(outline < 0 ? rows.length : outline + 1, 0, ...root.edgeMirrors)
        return rows.map((spec, index) => Object.assign({}, spec, { modelKey: "studio:" + index }))
    }

    // Alternatives offered for a piece whose resting face is a glyph. The description
    // names what is drawn on screen, so searching for the glyph finds the control.
    readonly property var pieceIcons: ({
        controls: { description: "The face of the bubble that opens the Control Center. It shows a wifi arc while Wi-Fi is on and a slider otherwise; pick a glyph here to keep one of your own instead.",
            glyphs: ["tune", "settings", "wifi", "toggle_on", "dashboard", "apps", "equalizer", "more_horiz"] },
        tools: { description: "The face of the Timers bubble while nothing is counting; a running timer always shows its ring and the minutes left.",
            glyphs: ["timer", "hourglass_empty", "alarm", "schedule", "av_timer", "timelapse"] },
        focus: { description: "The moon on the Do Not Disturb bubble. Silenced is shown by the filled disc behind it, so the glyph stays yours.",
            glyphs: ["bedtime", "dark_mode", "nightlight", "do_not_disturb_on", "notifications_off", "self_improvement"] },
        notifications: { description: "The bell on the notifications bubble. The unread count sits under it either way.",
            glyphs: ["notifications", "notifications_active", "campaign", "chat_bubble", "mark_email_unread", "inbox"] },
        bluetooth: { description: "The face of the Bluetooth bubble. Disabled and connected states keep their own glyphs unless you choose one here.",
            glyphs: ["bluetooth", "bluetooth_connected", "bluetooth_audio", "headphones", "devices", "cast"] },
        updates: { description: "The face of the Updates bubble while nothing is waiting; pending updates always show their count.",
            glyphs: ["task_alt", "system_update_alt", "deployed_code_update", "download", "package_2", "refresh"] },
        anime: { description: "The mark on the Airing bubble and its card. The next episode's cover replaces it on the bubble whenever there is one.",
            glyphs: ["live_tv", "smart_display", "subscriptions", "theaters", "animation", "movie"] },
        watching: { description: "The mark on the Continue bubble and its card. The bubble shows the episode number inside its progress ring once you have watched something.",
            glyphs: ["resume", "play_circle", "not_started", "slow_motion_video", "video_library", "history"] }
    })

    readonly property var bubbleRows: {
        const rows = []
        for (const kind of IrisPieces.iconKinds) {
            const icon = root.pieceIcons[kind]
            rows.push({ section: "bubbles", group: "Icons", label: IrisPieces.labelOf(kind) + " icon",
                description: icon.description, piece: kind, glyphs: icon.glyphs,
                path: "iris.pieces.icons", kind: "icon", fallback: "" })
        }
        for (const piece of IrisPieces.slots)
            rows.push({ section: "bubbles", group: "Placement", label: piece.label, description: piece.description,
                piece: piece.id,
                path: IrisPieces.configPath(piece.id) + ".place", kind: "zone", fallback: "island",
                choices: IrisPieces.zoneChoices(true) })
        for (const piece of IrisPieces.extras) {
            const path = IrisPieces.configPath(piece.id)
            // The two anime pieces live with the rest of anime in Settings; every
            // other bubble stays under Bubbles. One switch, one place.
            const anime = piece.id === "anime" || piece.id === "watching"
            const section = anime ? "anime" : "bubbles"
            const group = anime ? (piece.id === "anime" ? "Airing" : "Continue") : "Extra bubbles"
            const hint = IrisPieces.dependencyHint(piece.id)
            rows.push({ section: section, group: group, label: piece.label,
                description: hint.length > 0 ? hint : piece.description,
                piece: piece.id, keywords: piece.keywords ?? [],
                path: path + ".enable", kind: "switch", fallback: false })
            rows.push({ section: section, group: group, label: piece.label + " bubble rests",
                piece: piece.id,
                path: path + ".place", visibleWhen: path + ".enable", kind: "zone",
                fallback: IrisPieces.defaultPlace, choices: IrisPieces.zoneChoices(false) })
            if (piece.id === "workspaces") {
                rows.push({ section: "bubbles", group: "Workspaces", label: "Tap opens", description: "Open the compositor Overview, or grow a workspace switcher from this bubble.", piece: piece.id,
                    path: path + ".opens", visibleWhen: path + ".enable", kind: "choice", fallback: "card",
                    choices: [{label:"Overview",value:"overview",glyph:"space_dashboard"},{label:"Workspace card",value:"card",glyph:"view_carousel"}] })
            }
        }
        return rows
    }

    readonly property var behaviour: [
        { section: "appearance", group: "Look", label: "Base look", description: "The starting shape and contrast language. Customize tweaks stay on top of it.", path: "iris.appearance.preset", kind: "choice", fallback: "iris", choices: [{label:"Shima",value:"iris"},{label:"Soft",value:"soft"},{label:"Round",value:"round"},{label:"Crisp",value:"crisp"},{label:"Angular",value:"angular"},{label:"Contrast",value:"contrast"}] },
        { section: "bar", group: "Layout", label: "Composition", description: "Cluster splits media and controls into bubbles beside the clock.", path: "iris.bar.composition", kind: "choice", fallback: "cluster", choices: [{label:"Unified",value:"unified",glyph:"crop_7_5"},{label:"Cluster",value:"cluster",glyph:"bubble_chart"}] },
        { section: "bar", group: "Visibility", label: "Automatically hide", description: "The Island tucks into its edge and comes back when you rest the pointer there.", path: "iris.bar.autoHide", kind: "switch", fallback: false },
        { section: "bar", group: "Visibility", label: "Reserve space for windows", description: "Bubbles on the same edge reserve their own room. Everything that takes room is together in Windows, Room for Shima.", path: "iris.bar.reserveSpace", visibleWhen: "!iris.bar.autoHide", kind: "switch", fallback:true, keywords: ["reserve", "space", "windows", "exclusive", "margin", "espacio", "reservar"] },
        { section: "bar", group: "Interaction", label: "System events", description: "Connections, Do Not Disturb, Caps Lock and finished timers or recordings appear in the Island for a moment.", path: "iris.bar.events", kind: "switch", fallback: true },
        { section: "bar", group: "Connections", label: "Connection notices", description: "A moment in the Island when something is plugged in, connected, unplugged or lost.", path: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["connection", "connected", "disconnected", "plug", "unplug", "device", "notice", "toast", "popup", "wifi", "bluetooth", "usb", "charger", "offline"] },
        { section: "bar", group: "Connections", label: "Network", description: "Wi-Fi or a cable connected and dropped.", path: "osd.connections.network", visibleWhen: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["wifi", "ethernet", "cable"] },
        { section: "bar", group: "Connections", label: "Internet", description: "Lost and back online.", path: "osd.connections.internet", visibleWhen: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["offline", "online", "internet"] },
        { section: "bar", group: "Connections", label: "Bluetooth devices", description: "Headphones, keyboards, controllers.", path: "osd.connections.bluetooth", visibleWhen: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["bluetooth", "headphones", "airpods"] },
        { section: "bar", group: "Connections", label: "USB devices", description: "Mice, keyboards, controllers, cameras and phones, by name.", path: "osd.connections.usb", visibleWhen: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["usb", "mouse", "keyboard", "controller", "gamepad", "webcam", "camera", "phone"] },
        { section: "bar", group: "Connections", label: "Charger", description: "Plugged in and on battery.", path: "osd.connections.power", visibleWhen: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["charger", "charging", "battery", "power"] },
        { section: "bar", group: "Connections", label: "Sound output", description: "Headphones or speakers take over the sound.", path: "osd.connections.audio", visibleWhen: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["headphones", "speaker", "output", "sound"] },
        { section: "bar", group: "Connections", label: "Displays", description: "A monitor plugged in or removed.", path: "osd.connections.displays", visibleWhen: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["monitor", "display", "screen", "hdmi"] },
        { section: "bar", group: "Connections", label: "Drives and memory cards", description: "Pendrives, external disks and SD cards, with their name and size.", path: "osd.connections.drives", visibleWhen: "osd.connections.enable", kind: "switch", fallback: true, keywords: ["usb", "pendrive", "drive", "disk", "sd", "memory card", "external"] },
        { section: "bar", group: "Interaction", label: "Caps Lock badge", description: "A small pill drops out of the Island when Caps Lock turns on or off.", path: "keyboardIndicators.popup.caps", visibleWhen: "iris.bar.events", kind: "switch", fallback: true },
        { section: "bar", group: "Interaction", label: "Expand on hover", description: "The pointer has to rest on the Island; passing over it does nothing. A full-width Island always opens on a click instead.", path: "iris.bar.hoverExpand", kind: "switch", fallback:true },
        { section: "bar", group: "Interaction", label: "Hover delay", path: "iris.bar.hoverDelay", kind: "range", fallback:300,min:120,max:800,step:20,unit:" ms" },
        { section: "bar", group: "Interaction", label: "Scroll on the Island", description: "On the resting Island: Shift swaps volume and brightness; Ctrl adjusts the microphone. On the open Island scrolling never closes it: over the navigation row, or sideways anywhere, it moves between pages.", path: "iris.bar.scrollAction", kind: "choice", fallback: "volume", choices: [{label:"Volume",value:"volume"},{label:"Brightness",value:"brightness"},{label:"Off",value:"none"}] },
        { section: "bar", group: "Interaction", label: "Scroll on bubbles", description: "Also adjust over the media, controls and tray bubbles. Sound and Microphone bubbles always adjust their level.", path: "iris.bar.scrollBubbles", kind: "switch", fallback: true },
        { section: "bar", group: "Resting Island", label: "Trailing bubble", description: "The bubble beside a Cluster Island. Sound and Microphone show their level: scroll to adjust, click to mute.", visibleWhen: "iris.bar.composition=cluster", path: "iris.bar.trailing", kind: "choice", fallback: "controls", choices: [{label:"Controls",value:"controls"},{label:"Notifications",value:"notifications"},{label:"Weather",value:"weather"},{label:"Sound",value:"sound"},{label:"Microphone",value:"mic"},{label:"None",value:"none"}] },
        { section: "bar", group: "Resting Island", label: "Bubbles in the Island", description: "Small faces the Island carries itself, in the order you switch them on. Each one opens its card, and levels adjust on scroll. A bubble picked here leaves the frame; Bubbles › Extra bubbles floats one on the frame instead.", path: "iris.bar.pieces", kind: "pieces", fallback: [], choices: IrisPieces.extras.map(piece => ({ label: piece.label, value: piece.id })) },
        { section: "bar", group: "Resting Island", label: "Where they sit", description: "Which end of the Island carries them. On a side Island, before means above.", path: "iris.bar.piecesSide", kind: "choice", fallback: "end", choices: [{label:"After the clock",value:"end"},{label:"Before the clock",value:"start"}] },
        { section: "bar", group: "Bar", showIf: () => ["full", "menubar"].includes(String(Config.options?.iris?.bar?.layout ?? "island")), label: "Start", description: "What rests at the start of the full-width Island, in the order you switch it on. The Island itself is the live part: the time, or whatever is happening.", path: "iris.bar.fullStart", kind: "pieces", fallback: ["workspaces", "window"], choices: root.barZoneChoices },
        { section: "bar", group: "Bar", showIf: () => ["full", "menubar"].includes(String(Config.options?.iris?.bar?.layout ?? "island")), label: "Centre", description: "Kept in the middle of the edge while there is room.", path: "iris.bar.fullCenter", kind: "pieces", fallback: ["island"], choices: root.barZoneChoices },
        { section: "bar", group: "Bar", showIf: () => ["full", "menubar"].includes(String(Config.options?.iris?.bar?.layout ?? "island")), label: "End", description: "What rests at the far end. A piece that also floats on its own is left out here: one piece, one place.", path: "iris.bar.fullEnd", kind: "pieces", fallback: ["tray", "notifications", "sound", "controls"], choices: root.barZoneChoices },
        { section: "player", group: "Bubble", label: "Media bubble opens", description: "A card that floats out of the bubble, or the Island's player page. Cluster composition only.", path: "iris.player.bubbleOpens", kind: "choice", fallback: "card", choices: [{label:"Card",value:"card",glyph:"web_asset"},{label:"Island",value:"island",glyph:"pill"}] },
        { section: "player", group: "Bubble", label: "Keep the card open", description: "The card stays beside the Island while a player is active.", path: "iris.player.cardPinned", kind: "switch", fallback: false },
    ].concat(root.bubbleRows, [
        { section: "sources", group: "Weather", label: "Fahrenheit", description: "Degrees and distances in US units instead of Celsius and metric.", path: "bar.weather.useUSCS", kind: "switch", fallback: false },
        { section: "sources", group: "Weather", label: "Place", description: "The city the forecast is for. Leave it empty and Shima works it out from your connection, which is usually close but not always right.", path: "bar.weather.city", kind: "text", placeholder: "Worked out from your connection", fallback: "" },
        { section: "sources", group: "Weather", label: "Use GPS when there is one", description: "Asks geoclue instead of guessing from your connection. Off unless you want it.", path: "bar.weather.enableGPS", kind: "switch", fallback: false },
        { section: "sources", group: "Weather", label: "Check every", path: "bar.weather.fetchInterval", kind: "range", fallback: 10, min: 5, max: 60, step: 5, unit: " min" },
        { section: "sources", group: "Calendar", label: "Bring in other calendars", description: "Google and iCal feeds show up beside your own events, in the calendar bubble and in Today. Add the accounts themselves under More Settings, Services.", path: "calendar.externalSync.enable", kind: "switch", fallback: false },
        { section: "sources", group: "Calendar", label: "Refresh every", path: "calendar.externalSync.refreshMinutes", visibleWhen: "calendar.externalSync.enable", kind: "range", fallback: 15, min: 5, max: 120, step: 5, unit: " min" },
        { section: "sources", group: "Updates", label: "Look for updates every", description: "How often the Updates bubble counts what is waiting.", path: "updates.checkInterval", kind: "range", fallback: 120, min: 15, max: 720, step: 15, unit: " min" },
        { section: "sources", group: "VPN", label: "Connection details", description: "The VPN card shows addresses, the tailnet's devices and live traffic; tap a value to copy it.", path: "vpn.details", kind: "switch", fallback: false, keywords: ["vpn", "tailscale", "wireguard", "ip", "ipv4", "ipv6", "address", "devices", "traffic", "details", "detalles", "dirección"] },
        { section: "anime", group: "Airing", label: "Episodes in the card", description: "How many upcoming episodes the Airing bubble lists. Tap a show to follow it; the bubble shows what you follow first.", path: "iris.anime.shows", kind: "range", fallback: 5, min: 3, max: 8, keywords: ["anime", "weeb", "otaku", "airing", "episodes", "episodio", "following", "shows", "tracker"] },
        { section: "anime", group: "Continue", label: "Audio", description: "Japanese with subtitles, or dubbed where a dub exists. ani-cli only brings English subtitles and dubs.", path: "iris.anime.audio", visibleWhen: "iris.bubbles.extras.watching.enable", kind: "choice", fallback: "sub", choices: [{label:"Subtitled",value:"sub",glyph:"subtitles"},{label:"Dubbed",value:"dub",glyph:"record_voice_over"}], keywords: ["anime", "ani-cli", "dub", "doblaje", "doblado", "sub", "subtitles", "subtitulos", "subtítulos", "language", "idioma", "audio", "japanese", "english"] },
        { section: "anime", group: "Continue", label: "Quality", description: "The stream every episode starts in. The card can change it between episodes.", path: "iris.anime.quality", visibleWhen: "iris.bubbles.extras.watching.enable", kind: "choice", fallback: "best", choices: [{label:"Best",value:"best"},{label:"1080p",value:"1080"},{label:"720p",value:"720"},{label:"480p",value:"480"}], keywords: ["anime", "ani-cli", "quality", "calidad", "resolution", "resolución", "1080p", "720p", "480p", "stream", "video"] },
        { section: "anime", group: "Continue", label: "Subtitle size", description: "Every episode starts with subtitles this size. The Island's player changes it while you watch.", path: "iris.anime.subtitleScale", visibleWhen: "iris.bubbles.extras.watching.enable", kind: "range", fallback: 100, min: 50, max: 200, step: 10, unit: " %", keywords: ["anime", "subtitles", "subtítulos", "subtitulos", "size", "tamaño", "font", "letra", "mpv"] },
        { section: "anime", group: "Continue", label: "Subtitle height", description: "How high subtitles sit; 100 is the bottom edge.", path: "iris.anime.subtitlePosition", visibleWhen: "iris.bubbles.extras.watching.enable", kind: "range", fallback: 100, min: 50, max: 100, step: 1, unit: " %", keywords: ["anime", "subtitles", "subtítulos", "position", "posición", "height", "altura", "mpv"] },
        { section: "anime", group: "Continue", label: "Skip intros", description: "Jumps over openings where ani-skip knows them. Needs ani-skip installed.", path: "iris.anime.skipIntro", visibleWhen: "iris.bubbles.extras.watching.enable", kind: "switch", fallback: false, keywords: ["anime", "opening", "intro", "skip", "saltar", "ani-skip", "op", "ed"] },
        { section: "bubbles", group: "Behaviour", label: "Tapping a bubble", description: "Grow it into a card of its own, or open the Island page or panel it stands for (level bubbles then mute).", path: "iris.bubbles.opens", kind: "choice", fallback: "card", choices: [{label:"Opens its card",value:"card",glyph:"web_asset"},{label:"Opens the Island",value:"island",glyph:"pill"}] },
        { section: "bubbles", group: "On the contour", label: "Reserve space for windows", description: "Windows keep clear of the bubbles resting on an edge. Everything that takes room is together in Windows, Room for Shima.", path: "iris.bubbles.reserve", visibleWhen: "iris.bubbles.attach", kind: "switch", fallback: true, keywords: ["reserve", "space", "windows", "exclusive", "margin", "espacio", "reservar"] },
        { section: "bubbles", group: "Floating", label: "Space from the screen edges", description: "How far floating bubbles rest from the edges when they are not on the frame.", path: "iris.bubbles.edgeGap", visibleWhen: "!iris.bubbles.attach", kind: "range", fallback: 20, min: 0, max: 64, unit: " px" },
        { section: "bubbles", group: "Floating", label: "Snap to corners and edges", description: "Dropped near one, a bubble settles there; off, it stays where you let go.", path: "iris.bubbles.snap", kind: "switch", fallback: true },
        { section: "dock", group: "Visibility", label: "Show dock", path: "iris.dock.enable", kind: "switch", fallback:true },
        { section: "dock", group: "Visibility", label: "Automatically hide", description: "Rest the pointer at the screen edge to reveal it.", path: "iris.dock.autoHide", kind: "switch", fallback:true },
        { section: "dock", group: "Visibility", label: "Reserve space for windows", description: "Keep tiled windows clear of the visible Dock. Everything that takes room is together in Windows, Room for Shima.", path: "iris.dock.reserveSpace", visibleWhen: "!iris.dock.autoHide", kind: "switch", fallback:true, keywords: ["reserve", "space", "windows", "exclusive", "margin", "espacio", "reservar"] },
        { section: "dock", group: "Visibility", label: "Stay visible on empty workspaces", path: "iris.dock.revealOnEmpty", kind: "switch", fallback:true },
        { section: "dock", group: "Icons", label: "Notification badges", path: "iris.dock.badges", kind: "switch", fallback:true },
        { section: "motion", group: "Motion", label: "Reduce motion", description: "Surfaces appear in place. Shapes and joins stay the same.", path: "iris.appearance.motion", invert: true, kind: "switch", fallback: true },
        { section: "appearance", group: "Themes", label: "Take only their colours", description: "A theme you choose brings its accent, highlight, material colour and light, and leaves your shapes, layout and motion as they are.", path: "iris.appearance.themeColoursOnly", kind: "switch", fallback: false, keywords: ["theme", "colours", "colors", "palette", "only", "tema", "colores"] },
        { section: "appearance", group: "Customize", label: "Customize opens", description: "On the shell edits every surface where it is, with a capsule under the Island. Studio is a panel beside the screen, with every area in a column and one search.", path: "iris.appearance.customize", kind: "choice", fallback: "shell", choices: [{ label: "On the shell", value: "shell", glyph: "touch_app" }, { label: "Studio", value: "studio", glyph: "view_sidebar" }], keywords: ["studio", "customize", "editor", "panel", "look"] },
        { section: "appearance", group: "Previews", label: "Animated previews", description: "The live scenes and miniatures in Settings and Customize. Off, none of them is built, which saves memory and a little work while you browse.", path: "iris.appearance.previews", kind: "switch", fallback: true },
        { section: "spotlight", group: "Spotlight", label: "Spotlight", path: "iris.modules.palette", kind: "switch", fallback:true },
        { section: "spotlight", group: "Spotlight", label: "Maximum results", path: "iris.palette.maxResults", kind: "range", fallback:8,min:3,max:14 },
        { section: "spotlight", group: "Clipboard", label: "Entries shown", description: "How many recent copies Spotlight lists first; scroll or keep pressing Down to go through the whole history. Type ; to open it.", path: "iris.palette.clipboardResults", kind: "range", fallback: 8, min: 3, max: 20, keywords: ["clipboard", "history", "copy", "paste", "cliphist", "portapapeles", "entries", "limit"] },
        { section: "spotlight", group: "Spotlight", label: "Search mode shortcuts", description: "Clipboard, calculator, actions and more under the suggestions.", path: "iris.palette.showHints", kind: "switch", fallback:true },
        { section: "controlCenter", group: "Control Center", label: "Control Center", path: "iris.modules.controlCenter", kind: "switch", fallback:true },
        { section: "bubbles", group: "Tray", label: "Bubble shows", description: "Apps turns the bubble into the tray itself: every app is a disc you can click, right-click or scroll. Count keeps the number.", path: "iris.tray.face", kind: "choice", fallback: "apps", choices: [{label:"Apps",value:"apps",glyph:"apps"},{label:"Count",value:"count",glyph:"tag"}], keywords: ["tray", "icons", "background apps", "bubble", "count", "number", "system tray"] },
        { section: "bubbles", group: "Tray", label: "App names", path: "iris.tray.labels", kind: "switch", fallback:true },
        { section: "bubbles", group: "Tray", label: "Hide passive apps", path: "iris.tray.hidePassive", kind: "switch", fallback:false },
        { section: "bubbles", group: "Tray", label: "Columns", path: "iris.tray.columns", kind: "range", fallback:4,min:2,max:6 },
        { section: "desktop", group: "Wallpaper shuffle", label: "Shuffle wallpapers", description: "A new wallpaper from your folder every few minutes. Next wallpaper in the desktop menu does it once.", path: "background.autoWallpaper.enable", kind: "switch", fallback: false, keywords: ["shuffle", "random", "slideshow", "rotate", "cycle", "change", "automatic", "timer", "next wallpaper"] },
        { section: "desktop", group: "Wallpaper shuffle", label: "Every", path: "background.autoWallpaper.intervalMinutes", visibleWhen: "background.autoWallpaper.enable", kind: "range", fallback: 30, min: 1, max: 240, step: 1, unit: " min", keywords: ["interval", "minutes", "how often", "slideshow"] },
        { section: "desktop", group: "Wallpaper gallery", label: "Preview on the desktop", description: "The highlighted wallpaper shows behind the gallery; closing without applying restores yours.", path: "iris.wallpaper.livePreview", kind: "switch", fallback: true },
        { section: "desktop", group: "Wallpaper gallery", label: "Play previews", description: "Live wallpapers play a muted preview while chosen or hovered in the gallery. Only one plays at a time.", path: "iris.wallpaper.motion", kind: "switch", fallback: true },
        { section: "desktop", group: "Wallpaper gallery", label: "Library order", description: "How your folders line up in the gallery.", path: "iris.wallpaper.sort", kind: "choice", fallback: "newest", choices: [{label:"Newest",value:"newest",glyph:"schedule"},{label:"Oldest",value:"oldest",glyph:"history"},{label:"Name",value:"name",glyph:"sort_by_alpha"}], keywords: ["sort", "order", "date", "recent", "newest", "oldest", "name", "alphabetical"] },
        { section: "desktop", group: "Wallpaper gallery", label: "Online order", description: "Top brings the most liked first, Newest the latest uploads, Random a fresh mix. Live wallpapers keep the site's order.", path: "iris.wallpaper.onlineSort", kind: "choice", fallback: "top", choices: [{label:"Top",value:"top",glyph:"trending_up"},{label:"Newest",value:"newest",glyph:"schedule"},{label:"Random",value:"random",glyph:"shuffle"}], keywords: ["sort", "order", "top", "popular", "toplist", "newest", "latest", "random", "wallhaven", "konachan", "yande.re"] },
        { section: "desktop", group: "Wallpaper gallery", label: "Top of the", description: "How far back Top looks on Wallhaven, Konachan and yande.re.", path: "iris.wallpaper.topRange", visibleWhen: "iris.wallpaper.onlineSort=top", kind: "choice", fallback: "1w", choices: [{label:"Day",value:"1d"},{label:"Week",value:"1w"},{label:"Month",value:"1M"},{label:"Year",value:"1y"}], keywords: ["top", "weekly", "monthly", "daily", "yearly", "day", "week", "month", "year", "period", "range", "toplist"] },
        { section: "desktop", group: "Wallpaper gallery", label: "Online sources", description: "Where the gallery looks beyond your folders, in the order you turn them on. Konachan and yande.re lean to fan service even when rated safe.", path: "iris.wallpaper.sources", kind: "pieces", fallback: ["wallhaven", "live"], choices: [{label:"Wallhaven",value:"wallhaven"},{label:"Live",value:"live"},{label:"Konachan",value:"konachan"},{label:"yande.re",value:"yandere"}], keywords: ["source", "provider", "wallhaven", "konachan", "yande.re", "yandere", "motionbgs", "live", "online", "booru", "download"] },
        { section: "desktop", group: "Wallpaper gallery", label: "Suggestive art", description: "Off keeps swimsuits, lingerie and fan service out of online results. Explicit art never shows.", path: "iris.wallpaper.suggestive", kind: "switch", fallback: false, keywords: ["nsfw", "sfw", "safe", "ecchi", "fan service", "suggestive", "filter", "family", "work", "adult", "content"] },
        { section: "desktop", group: "Wallpaper gallery", label: "Live wallpaper downloads", description: "Best downloads the 4K file and plays a copy sized to your screen; the HD files are heavily compressed. Light takes HD: a fifth of the download, visibly softer.", path: "iris.wallpaper.liveQuality", kind: "choice", fallback: "best", choices: [{label:"Best",value:"best"},{label:"Light",value:"light"}], keywords: ["4k", "hd", "quality", "resolution", "video", "motionbgs"] },
        { section: "desktop", group: "Live wallpapers", label: "Animate on the desktop", description: "Video and GIF wallpapers move. Off keeps a sharp still frame and no decoder.", path: "background.enableAnimation", kind: "switch", fallback: true },
        { section: "desktop", group: "Live wallpapers", label: "Pause", description: "A hidden wallpaper stops decoding and keeps its frame. When covered pauses once tiled windows fill the screen; Never keeps it moving under everything.", path: "background.videoPause", kind: "choice", fallback: "covered", choices: [{label:"Never",value:"never",glyph:"play_arrow"},{label:"Behind fullscreen",value:"fullscreen",glyph:"fullscreen"},{label:"When covered",value:"covered",glyph:"view_column"}] },
        { section: "notifications", group: "Banners", label: "Notification banners", path: "iris.modules.notificationPopup", kind: "switch", fallback:true },
        { section: "notifications", group: "Banners", label: "Banner duration", description: "How long a notification stays when the app does not choose. Hovering keeps it.", path: "iris.notifications.duration", kind: "range", fallback:4000,min:2000,max:12000,step:500,scale:0.001,unit:" s" },
        { section: "sound", group: "Feedback", label: "On-screen feedback", description: "Volume, brightness, song and keyboard changes show on the Island, or in a small pill when it is hidden.", path: "iris.modules.osd", kind: "switch", fallback:true, keywords: ["osd", "volume", "brightness", "popup", "indicator"] },
        { section: "sound", group: "Feedback", label: "Volume, brightness and mic", description: "Yours shows what you change with keys, scrolling or sliders; Every change also shows apps and devices moving them, like voice chat adjusting the mic.", path: "iris.osd.levels", visibleWhen: "iris.modules.osd", kind: "choice", fallback: "yours", choices: [{label:"Yours",value:"yours",glyph:"touch_app"},{label:"Every change",value:"every",glyph:"all_inclusive"},{label:"Never",value:"off",glyph:"block"}], keywords: ["osd", "volume", "brightness", "microphone", "mic", "level", "automatic"] },
        { section: "sound", group: "Feedback", label: "Song changes", description: "Yours shows the song when you skip or pause; Every change also shows when the next one starts by itself.", path: "iris.osd.media", visibleWhen: "iris.modules.osd", kind: "choice", fallback: "yours", choices: [{label:"Yours",value:"yours",glyph:"touch_app"},{label:"Every change",value:"every",glyph:"all_inclusive"},{label:"Never",value:"off",glyph:"block"}], keywords: ["osd", "music", "track", "song", "now playing", "media", "next", "skip"] },
        { section: "sound", group: "Feedback", label: "Keyboard layout and locks", path: "iris.osd.keyboard", visibleWhen: "iris.modules.osd", kind: "switch", fallback: true, keywords: ["osd", "caps lock", "num lock", "layout", "language", "keyboard"] },
        { section: "gaming", group: "Fullscreen", label: "Notification banners", description: "Over a fullscreen video or game. Off keeps them away; they still collect in Today.", path: "iris.notifications.fullscreen", visibleWhen: "iris.modules.notificationPopup", kind: "switch", fallback: true, keywords: ["fullscreen", "video", "movie", "banner", "notification"] },
        { section: "gaming", group: "Fullscreen", label: "Volume and song pill", description: "Quiet keeps volume, brightness and mic and drops songs and keyboard; Hidden shows nothing.", path: "iris.osd.fullscreen", visibleWhen: "iris.modules.osd", kind: "choice", fallback: "show", choices: [{label:"Everything",value:"show",glyph:"visibility"},{label:"Quiet",value:"quiet",glyph:"volume_down"},{label:"Hidden",value:"hide",glyph:"visibility_off"}], keywords: ["osd", "game", "gaming", "gamemode", "fullscreen", "video", "quiet"] },
        { section: "sound", group: "Volume", label: "Ear protection", description: "Holds back sudden jumps and keeps the volume under the limit. Shared with Material and Waffle.", path: "audio.protection.enable", kind: "switch", fallback: true, keywords: ["earbang", "volume", "limit", "loud", "hearing", "safety"] },
        { section: "sound", group: "Volume", label: "Largest jump", description: "The most one change can raise the volume.", path: "audio.protection.maxAllowedIncrease", visibleWhen: "audio.protection.enable", kind: "range", fallback: 10, min: 2, max: 50, step: 2, unit: " %", keywords: ["earbang", "volume", "step"] },
        { section: "sound", group: "Volume", label: "Volume limit", description: "Above 100 % amplifies the sound and can distort it; 153 % is as far as the system mixer goes.", path: "audio.protection.maxAllowed", visibleWhen: "audio.protection.enable", kind: "range", fallback: 100, min: 50, max: 154, step: 2, unit: " %", keywords: ["earbang", "volume", "limit", "boost", "amplify", "over 100", "150"] },
    ].concat(...["left", "right"].map(side => [
        { section: "sidebars", group: side === "left" ? "Focus · left" : "Today · right", label: "Enable panel", path: "iris.sidebars." + side + ".enable", kind: "switch", fallback:true },
        { section: "sidebars", group: side === "left" ? "Focus · left" : "Today · right", label: "Width", path: "iris.sidebars." + side + ".width", kind: "range", fallback:380,min:300,max:600,step:10,unit:" px" },
        { section: "sidebars", group: side === "left" ? "Focus · left" : "Today · right", label: "Maximum height", description: "Panels hug their sections and grow up to this.", path: "iris.sidebars." + side + ".height", kind: "range", fallback:88,min:45,max:100,unit:" %" },
        { section: "sidebars", group: side === "left" ? "Focus · left" : "Today · right", label: "Alignment", path: "iris.sidebars." + side + ".alignment", kind: "choice", fallback:"center", choices:[{label:"Top",value:"top"},{label:"Center",value:"center"},{label:"Bottom",value:"bottom"}] },
        { section: "sidebars", group: side === "left" ? "Focus · left" : "Today · right", label: "Attach to the screen edge", description: "Melts the panel into its edge, like a notch. Off, it floats beside it.", path: "iris.sidebars." + side + ".notch", kind: "switch", fallback:false, keywords: ["attach", "join", "frame", "edge", "notch", "float", "pegar"] },
        { section: "sidebars", group: side === "left" ? "Focus · left" : "Today · right", label: "Reveal on hover", description: side === "left" ? "Rest the pointer at the left edge to peek; click inside to keep it." : "Rest the pointer at the right edge to peek; click inside to keep it.", path: "iris.sidebars." + side + ".hoverReveal", kind: "switch", fallback:false },
        { section: "sidebars", group: side === "left" ? "Focus · left" : "Today · right", label: "Keep open", description: "Stay visible while working in other windows.", path: "iris.sidebars." + side + ".pinned", kind: "switch", fallback:false },
        { section: "sidebars", group: side === "left" ? "Focus · left" : "Today · right", label: "Reserve space for windows", description: "Keep tiled windows clear while this panel is kept open. Turn this off to let the panel overlay them.", path: "iris.sidebars." + side + ".reserveSpace", visibleWhen: "iris.sidebars." + side + ".pinned", kind: "switch", fallback:true, keywords: ["reserve", "space", "windows", "exclusive", "margin", "espacio", "reservar"] }
    ])))

    readonly property var idleChoices: [
        { label: "Never", value: 0 }, { label: "1 min", value: 60 }, { label: "2 min", value: 120 },
        { label: "5 min", value: 300 }, { label: "10 min", value: 600 }, { label: "15 min", value: 900 },
        { label: "30 min", value: 1800 }, { label: "1 hour", value: 3600 }
    ]
    // Shared iNiR keys an iRiS surface acts on, so they are set where iRiS shows them. Material and
    // Waffle read the same keys; nothing here is copied.
    readonly property var shared: [
        { section: "general", group: "Date & time", label: "Clock", description: "The Island, the lock screen and every clock in Shima.", path: "time.format", kind: "choice", fallback: "h:mm AP", choices: [{label:"24-hour",value:"HH:mm"},{label:"12-hour AM/PM",value:"h:mm AP"}], keywords: ["time", "hour", "24", "12", "am", "pm", "hora"] },
        { section: "general", group: "Date & time", label: "Exact seconds", description: "Clocks that show seconds tick on the second. Costs a little more.", path: "time.secondPrecision", kind: "switch", fallback: false },
        { section: "general", group: "Language", label: "Language", description: "Every label in the shell. System follows your locale.", path: "language.ui", kind: "choice", fallback: "auto",
            choices: [{ label: "System", value: "auto" }].concat(Translation.allAvailableLanguages.map(code => ({ label: Translation.languageDisplayName(code), value: code }))),
            keywords: ["language", "idioma", "translation", "locale", "español", "spanish"] },
        { section: "general", group: "Apps", label: "Terminal", description: "Where Spotlight's commands, package installs and anime playback open.", path: "apps.terminal", kind: "text", placeholder: "kitty", fallback: "kitty", keywords: ["terminal", "kitty", "foot", "alacritty", "ghostty", "wezterm", "default apps"] },
        { section: "general", group: "Settings window", label: "Layout", description: "Sidebar lists every area beside the page. Rail keeps only their marks, for more room. Home opens on every area at once; each opens full width.", path: "iris.appearance.settingsLayout", kind: "choice", fallback: "sidebar", choices: [{ label: "Sidebar", value: "sidebar", glyph: "view_sidebar" }, { label: "Rail", value: "rail", glyph: "view_week" }, { label: "Home", value: "home", glyph: "grid_view" }], keywords: ["settings", "navigation", "sidebar", "rail", "grid", "home", "layout"] },
        { section: "general", group: "Performance", label: "Low power", description: "Turns glass, blur and heavy wallpaper effects off everywhere.", path: "performance.lowPower", kind: "switch", fallback: false, keywords: ["performance", "battery", "effects", "glass", "blur", "rendimiento"] },

        { section: "spotlight", group: "Web search", label: "Search the web with", description: "What ? and Search the web use in Spotlight.", path: "search.engineBaseUrl", kind: "choice", fallback: "https://www.google.com/search?q=",
            choices: [{label:"Google",value:"https://www.google.com/search?q="},{label:"DuckDuckGo",value:"https://duckduckgo.com/?q="},{label:"Brave",value:"https://search.brave.com/search?q="},{label:"Bing",value:"https://www.bing.com/search?q="},{label:"Startpage",value:"https://www.startpage.com/do/search?q="}],
            keywords: ["search engine", "google", "duckduckgo", "brave", "bing", "startpage", "buscador"] },



        { section: "notifications", group: "Do Not Disturb", label: "Do Not Disturb", description: "Banners stay away; notifications still collect in Today.", path: "notifications.silent", kind: "switch", fallback: false, keywords: ["dnd", "silent", "focus", "quiet", "mute"] },

        { section: "sound", group: "Alert sounds", label: "Alert volume", description: "Notifications, timers and battery warnings.", path: "sounds.volume", kind: "range", fallback: 0.5, min: 0, max: 1, step: 0.05, scale: 100, unit: " %" },
        { section: "sound", group: "Alert sounds", label: "Timers", path: "sounds.timer", kind: "switch", fallback: false },
        { section: "sound", group: "Alert sounds", label: "Focus sessions", path: "sounds.pomodoro", kind: "switch", fallback: false, keywords: ["pomodoro"] },




        { section: "display", group: "Night Light", label: "On a schedule", description: "Warmer from sunset to sunrise where you are.", path: "light.night.automatic", nightLight: "schedule", kind: "switch", fallback: true, keywords: ["night", "blue light", "warm", "wlsunset", "noche"] },
        { section: "display", group: "Night Light", label: "Night Light now", path: "light.night.enabled", nightLight: "on", kind: "switch", fallback: false },
        { section: "display", group: "Night Light", label: "Warmth", description: "Lower is warmer.", path: "light.night.colorTemperature", nightLight: "warmth", kind: "range", fallback: 5000, min: 2500, max: 6500, step: 100, unit: " K" },
        { section: "display", group: "Comfort", label: "Soften bright flashes", description: "Dims the screen when a mostly white window suddenly fills it.", path: "light.antiFlashbang.enable", kind: "switch", fallback: false, keywords: ["flashbang", "bright", "white", "eyes"] },

        { section: "battery", group: "Warnings", label: "Low battery", path: "battery.low", kind: "range", fallback: 20, min: 5, max: 50, step: 5, unit: " %" },
        { section: "battery", group: "Warnings", label: "Critical battery", path: "battery.critical", kind: "range", fallback: 5, min: 1, max: 20, unit: " %" },
        { section: "battery", group: "Warnings", label: "Warning sounds", path: "sounds.battery", kind: "switch", fallback: false },
        { section: "battery", group: "Suspend", label: "Suspend when almost empty", path: "battery.automaticSuspend", kind: "switch", fallback: true },
        { section: "battery", group: "Suspend", label: "At", path: "battery.suspend", visibleWhen: "battery.automaticSuspend", kind: "range", fallback: 3, min: 1, max: 10, unit: " %" },
        { section: "battery", group: "Charge limit", label: "Limit charging", description: "Stops charging early so the battery ages slower. Needs a laptop that supports it.", path: "battery.chargeLimit.enable", kind: "switch", fallback: false },
        { section: "battery", group: "Charge limit", label: "Stop at", path: "battery.chargeLimit.threshold", visibleWhen: "battery.chargeLimit.enable", kind: "range", fallback: 80, min: 50, max: 100, step: 5, unit: " %" },

        { section: "gaming", group: "Game mode", label: "Turn on for fullscreen apps", description: "Game mode starts by itself while a window is fullscreen and ends when it leaves.", path: "gameMode.autoDetect", kind: "switch", fallback: true, keywords: ["game", "gaming", "fullscreen", "juego", "performance"] },
        { section: "gaming", group: "Game mode", label: "Stop shell animations", description: "Shima shapes appear in place while you play.", path: "gameMode.disableAnimations", kind: "switch", fallback: true },
        { section: "gaming", group: "Game mode", label: "Stop glass and blur", path: "gameMode.disableEffects", kind: "switch", fallback: true },
        { section: "gaming", group: "Game mode", label: "Hold notifications", description: "Banners wait until you are done; they still collect in Today.", path: "gameMode.suppressNotifications", kind: "switch", fallback: true },
        { section: "gaming", group: "Game mode", label: "Stop audio visualizers", path: "gameMode.disableVisualizers", kind: "switch", fallback: true },
        { section: "gaming", group: "Game mode", label: "Close the Discord overlay", path: "gameMode.disableDiscoverOverlay", kind: "switch", fallback: true },
        { section: "gaming", group: "Game mode", label: "Hide reload notices", description: "The small notice when your config reloads stays away while you play.", path: "gameMode.disableReloadToasts", kind: "switch", fallback: true, keywords: ["toast", "reload"] },

    ]

    readonly property var swatchChoices: [
        { label: "Accent", value: String(IrisStyle.accent), get swatch() { return IrisStyle.accent } },
        { label: "Highlight", value: String(IrisStyle.secondaryAccent), get swatch() { return IrisStyle.secondaryAccent } },
        { label: "Blue", value: String(IrisStyle.identity.blue), get swatch() { return IrisStyle.identity.blue } },
        { label: "Green", value: String(IrisStyle.identity.green), get swatch() { return IrisStyle.identity.green } },
        { label: "Pink", value: String(IrisStyle.identity.pink), get swatch() { return IrisStyle.identity.pink } },
        { label: "Gray", value: String(IrisStyle.identity.gray), get swatch() { return IrisStyle.identity.gray } }
    ]
    readonly property var dockEdgeChoices: [{ label: "Auto", value: "auto", glyph: "auto_mode" }].concat([
        { label: "Bottom", value: "bottom", glyph: "vertical_align_bottom" }, { label: "Top", value: "top", glyph: "vertical_align_top" },
        { label: "Left", value: "left", glyph: "align_horizontal_left" }, { label: "Right", value: "right", glyph: "align_horizontal_right" }
    ])
    readonly property var barZoneChoices: [
        { label: "Island", value: "island" }, { label: "Workspaces", value: "workspaces" },
        { label: "Current window", value: "window" }, { label: "Time", value: "time" }
    ].concat(IrisPieces.extras.filter(piece => piece.id !== "workspaces").map(piece => ({ label: piece.label, value: piece.id })))
    // Settings' sidebar, in the order and clusters it shows. `tip` is the one line a section opens on.
    readonly property var sections: [
        { id: "general", cluster: 0, title: "General", subtitle: "Time, language and how Shima behaves", icon: "settings", get tint() { return IrisStyle.identity.gray }, tip: "Settings that shape the whole shell, whichever surface shows them." },
        { id: "appearance", cluster: 0, title: "Appearance", subtitle: "Material, colour, type and shape", icon: "palette", get tint() { return IrisStyle.identity.purple }, tip: "Customize edits the same values live, on top of your desktop." },
        { id: "motion", cluster: 0, title: "Motion", subtitle: "How every shape grows, moves and settles", icon: "animation", get tint() { return IrisStyle.identity.indigo }, tip: "Shorter durations feel snappier; the curve stays the same." },
        { id: "bar", cluster: 1, title: "Island", subtitle: "Composition, size and how the Island responds", icon: "pill", get tint() { return IrisStyle.identity.blue }, tip: "Rest on the Island to peek, click to keep it, scroll for volume." },
        { id: "bubbles", cluster: 1, title: "Bubbles & Cards", subtitle: "Pieces you carry and the cards they grow", icon: "bubble_chart", get tint() { return IrisStyle.identity.sky }, tip: "Hold a bubble to carry it; drop it beside the Island to bring it back." },
        { id: "dock", cluster: 1, title: "Dock", subtitle: "Visibility, material and app icons", icon: "dock_to_bottom", get tint() { return IrisStyle.identity.indigo }, tip: "Right-click an icon for its windows or to float it as a bubble; middle-click opens a new one." },
        { id: "desktop", cluster: 1, title: "Desktop & Wallpaper", subtitle: "Widgets, wallpapers and the overview backdrop", icon: "wallpaper", get tint() { return IrisStyle.identity.teal }, tip: "Right-click the desktop to edit widgets." },
        { id: "sidebars", cluster: 1, title: "Side Panels", subtitle: "Focus and Today, arranged around your workflow", icon: "dock_to_right", get tint() { return IrisStyle.identity.green }, tip: "Ctrl+E customizes a panel; Keep open makes room beside windows." },
        { id: "controlCenter", cluster: 2, title: "Control Center", subtitle: "What it carries and how it looks", icon: "toggle_on", get tint() { return IrisStyle.identity.gray }, tip: "Arrange it in place from the button beside the lock in its header." },
        { id: "spotlight", cluster: 2, title: "Spotlight", subtitle: "Search apps, files, the clipboard and more", icon: "search", get tint() { return IrisStyle.identity.blue }, tip: "Prefixes: ; clipboard, = calculator, / actions." },
        { id: "notifications", cluster: 2, title: "Notifications", subtitle: "Banners, quiet hours and Do Not Disturb", icon: "notifications", get tint() { return IrisStyle.identity.red }, tip: "Hover a banner to keep it; swipe it away to dismiss." },
        { id: "sound", cluster: 2, title: "Sound & Feedback", subtitle: "Volume, alert sounds and what the OSD shows", icon: "volume_up", get tint() { return IrisStyle.identity.pink }, tip: "Scroll on the Island for volume; Ctrl for the microphone." },
        { id: "display", cluster: 3, title: "Display", subtitle: "Resolution, scale, night light and bright flashes", icon: "brightness_6", get tint() { return IrisStyle.identity.blue }, tip: "Shift and scroll on the Island for brightness." },
        { id: "battery", cluster: 3, title: "Battery", subtitle: "Warnings, suspend and charge limit", icon: "battery_full", get tint() { return IrisStyle.identity.green }, tip: "Only matters on a machine with a battery." },
        { id: "gaming", cluster: 3, title: "Gaming & Fullscreen", subtitle: "What Shima steps back from while you play", icon: "sports_esports", get tint() { return IrisStyle.identity.purple }, tip: "Game mode turns on by itself for fullscreen games." },
        { id: "player", cluster: 4, title: "Now Playing", subtitle: "Music in the Island and on the lock screen", icon: "music_note", get tint() { return IrisStyle.identity.pink }, tip: "Middle-click the Island to play or pause." },
        { id: "frameMusic", cluster: 4, title: "Frame Music", subtitle: "Shape and response of the music-driven frame", icon: "graphic_eq", get tint() { return IrisStyle.identity.pink }, tip: "Switch Music on the edges on to make the screen contour move with your music." },
        { id: "anime", cluster: 4, title: "Anime", subtitle: "Airing, Continue and the anime colour layer", icon: "live_tv", get tint() { return IrisStyle.identity.purple }, tip: "Airing tracks what is coming; Continue resumes what you were watching." },
        { id: "sources", cluster: 5, title: "Sources", subtitle: "Where the Island gets what it shows", icon: "cloud_sync", get tint() { return IrisStyle.identity.yellow }, tip: "A setting belongs here when a Shima surface puts its data on screen." },
            ]
    // The shared iNiR pages that still act under iRiS, as "More Settings" lists them. Pages for other families stay out.
    readonly property var morePages: [
        { group: "Desktop", key: "wallpaper", label: "Wallpaper engine", detail: "Backend, transitions, rotation", icon: "wallpaper", get tint() { return IrisStyle.identity.teal }, keywords: ["wallpaper", "awww", "transition", "slideshow", "fondo"] },
        { group: "Desktop", key: "widgets", label: "Desktop widgets", detail: "Every widget's own options", icon: "widgets", get tint() { return IrisStyle.identity.sky }, keywords: ["widgets", "clock", "weather", "media"] },
        { group: "Desktop", key: "monitors", label: "Monitors", detail: "What each screen shows", icon: "display_settings", get tint() { return IrisStyle.identity.blue }, keywords: ["monitor", "screen", "output", "display", "pantalla"] },
        { group: "System", key: "autostart", label: "Autostart", detail: "Apps that start with you", icon: "rocket_launch", get tint() { return IrisStyle.identity.orange }, keywords: ["autostart", "startup", "login", "inicio"] },
        { group: "System", key: "services", label: "Services", detail: "Search, network and data", icon: "settings_suggest", get tint() { return IrisStyle.identity.gray }, keywords: ["services", "network", "search", "data"] },
        { group: "System", key: "tools", label: "Tools", detail: "Recording codecs and snip details", icon: "build", get tint() { return IrisStyle.identity.red }, keywords: ["tools", "codec", "ffmpeg", "vaapi", "discord", "crosshair"] },
        { group: "Shell", key: "themes", label: "Colour themes", detail: "Every palette, with previews; app icon theme", icon: "palette", get tint() { return IrisStyle.identity.purple }, keywords: ["theme", "palette", "colours", "catppuccin", "apps", "icons", "icon theme", "icon pack", "papirus", "tray icons", "iconos"] },
        { group: "Shell", key: "system", label: "System", detail: "Keyboard indicators, work safety", icon: "browse", get tint() { return IrisStyle.identity.green }, keywords: ["system", "keyboard", "caps lock", "safety"] },
        { group: "Shell", key: "advanced", label: "Advanced", detail: "Visualizer and colour generation", icon: "construction", get tint() { return IrisStyle.identity.gray }, keywords: ["advanced", "cava", "visualizer", "matugen"] }
    ]
    readonly property var sectionOrder: root.sections.map(section => section.id)
    // Customize's areas (Studio's rail, the capsule's sheets): identity, what each is, one line worth knowing.
    readonly property var customizeAreas: [
        { id: "material", label: "Material", glyph: "layers", get tint() { return IrisStyle.identity.purple }, tipOf: "", tip: "Character sets corners, fills and contrast at once; the rows below fine-tune it.",
            about: "What every surface is made of: its character, corners, lines and the frame." },
        { id: "colour", label: "Colour", glyph: "palette", get tint() { return IrisStyle.identity.pink }, tipOf: "", tip: "A wallpaper accent follows every new wallpaper.",
            about: "Accent, highlight, the light bodies carry and how much wallpaper Shima takes in." },
        { id: "type", label: "Type", glyph: "text_fields", get tint() { return IrisStyle.identity.orange }, tipOf: "", tip: "Figures have their own face: the clock and every level read it.",
            about: "Typefaces, figures and how large text reads." },
        { id: "motion", label: "Motion", glyph: "animation", get tint() { return IrisStyle.identity.indigo }, tipOf: "motion",
            about: "How shapes open, move and settle, everywhere at once or per surface." },
        { id: "island", label: "Island", glyph: "pill", get tint() { return IrisStyle.identity.blue }, tipOf: "bar",
            about: "Its shape on the edge, what it shows at rest, and its pages." },
        { id: "pieces", label: "Pieces", glyph: "bubble_chart", get tint() { return IrisStyle.identity.sky }, tipOf: "bubbles",
            about: "Bubbles off the Island and the bars they form." },
        { id: "bodies", label: "Cards", glyph: "web_asset", get tint() { return IrisStyle.identity.teal }, tipOf: "controlCenter",
            about: "Cards, the Control Center and the player." },
        { id: "places", label: "Panels", glyph: "space_dashboard", get tint() { return IrisStyle.identity.green }, tipOf: "sidebars",
            about: "Side panels, Spotlight, the gallery, Settings and menus." },
        { id: "transients", label: "Feedback", glyph: "notifications", get tint() { return IrisStyle.identity.red }, tipOf: "notifications",
            about: "Notifications and level feedback." },
        { id: "dock", label: "Dock", glyph: "dock_to_bottom", get tint() { return IrisStyle.identity.lavender }, tipOf: "dock",
            about: "The Dock's shape and its icons." },
        { id: "desktop", label: "Desktop", glyph: "widgets", get tint() { return IrisStyle.identity.yellow }, tipOf: "desktop",
            about: "Widgets on the desktop." },
        { id: "themes", label: "Themes", glyph: "style", get tint() { return IrisStyle.identity.gray }, tipOf: "", tip: "A theme is one small file: copy it to share, paste one to try it.",
            about: "Whole redesigns of Shima, and the ones you save and share." }
    ]
    function customizeArea(id: string): var { return root.customizeAreas.find(area => area.id === id) ?? root.customizeAreas[0] }
    function customizeTip(id: string): string {
        const area = root.customizeArea(id)
        if (area.tip) return Translation.tr(area.tip)
        return area.tipOf.length > 0 ? Translation.tr(root.sectionById(area.tipOf).tip ?? "") : ""
    }
    function sectionById(id: string): var {
        return root.sections.find(section => section.id === id) ?? root.sections[0]
    }
    readonly property var groupGlyphs: ({
        "Shell family": "swap_horiz", "Notch": "vertical_align_top",
        "Wallpaper shuffle": "shuffle",
        "Behind windows": "blur_on", "Japanese lookup": "translate", "Parallax": "3d_rotation", "Recording": "screen_record", "Snip": "screenshot_region",
        "Accent": "palette", "Visualizer": "graphic_eq", "Customize": "brush", "Themes": "style", "Settings window": "settings_applications", "Activity": "timer", "Adaptive": "auto_awesome", "App colours": "format_paint", "Airing": "live_tv", "Alert sounds": "music_note",
        "At a glance": "visibility", "At rest": "schedule", "Badges": "notifications_unread", "Banners": "notifications",
        "Bar": "width_full", "Behaviour": "touch_app", "Bubble": "bubble_chart", "Calendar": "calendar_month",
        "Card contents": "view_agenda", "Cards": "web_asset", "Charge limit": "battery_charging_80", "Clipboard": "content_paste", "Connections": "cable", "Clock": "schedule",
        "Colour layer": "palette", "Comfort": "visibility", "Continue": "play_circle", "Control Center": "tune",
        "Curve": "show_chart", "Date & time": "calendar_clock", "Desktop page": "dashboard", "Do Not Disturb": "do_not_disturb_on",
        "Extra bubbles": "add_circle", "Faces": "font_download", "Feedback": "campaign", "Floating": "flight",
        "Focus · left": "dock_to_left", "Frame": "crop_free", "Frame response": "graphic_eq", "On the edges": "border_outer", "Finish": "flare", "Fullscreen": "fullscreen", "Game mode": "sports_esports",
        "Glass": "blur_on", "Highlight": "highlight", "Icons": "emoji_symbols", "Interaction": "ads_click", "Joining": "join_inner",
        "Language": "language", "Layout": "view_quilt", "Light": "light_mode", "Live wallpapers": "motion_photos_on",
        "Desktop menu": "menu_open", "Look": "visibility", "Material": "layers", "Material per surface": "layers", "Corners per surface": "rounded_corner", "Colour theme": "palette", "Scheme": "contrast", "Dark look": "dark_mode", "Ink look": "ink_pen", "Light look": "light_mode", "Menus": "menu", "Motion": "animation",
        "Night Light": "nightlight", "Notifications": "notifications", "Now playing": "music_note", "On the contour": "border_outer",
        "Opening bodies": "open_in_full", "Overview backdrop": "grid_view", "Pages": "view_carousel", "Per surface": "tune",
        "Panel look": "dock_to_right", "Placement": "location_on", "Player": "music_note", "Player page": "album", "Previews": "preview",
        "Resting Island": "pill", "Scene": "landscape", "Security": "key", "Settings": "settings", "Shape": "rounded_corner",
        "Sign in": "person", "Size": "straighten", "Sound": "volume_up", "Spotlight": "search",
        "Status": "signal_cellular_alt", "Style per surface": "motion_mode", "Suspend": "bedtime", "Text": "text_fields", "Timing": "timer", "Today · right": "dock_to_right",
        "Touch": "touch_app", "Tray": "inventory_2", "Type": "text_fields", "Updates": "system_update_alt", "Visibility": "visibility",
        "Volume": "volume_up", "VPN": "vpn_lock", "Wallpaper": "wallpaper", "Wallpaper gallery": "photo_library",
        "Warnings": "battery_alert", "Weather": "partly_cloudy_day", "When idle": "hourglass_empty", "Widgets": "widgets",
        "Windows": "select_window", "Workspaces": "view_column", "Apps": "apps", "Performance": "speed", "Web search": "travel_explore",
        "Read text": "document_scanner", "Borders": "border_style", "Edges": "border_style", "Movement": "animation", "Closing": "close",
        "Room for Shima": "fit_screen", "Keyboard": "keyboard", "Mouse": "mouse", "Trackpad": "touch_app", "Pointer": "arrow_selector_tool", "Screen": "monitor"
    })
    readonly property var groupTints: ({
        get "Shell family"() { return IrisStyle.identity.purple },
        get "Notch"() { return IrisStyle.identity.blue },
        get "Accent"() { return IrisStyle.identity.blue },
        get "Activity"() { return IrisStyle.identity.orange },
        get "Adaptive"() { return IrisStyle.identity.teal },
        get "Airing"() { return IrisStyle.identity.purple },
        get "Alert sounds"() { return IrisStyle.identity.pink },
        get "App colours"() { return IrisStyle.identity.orange },
        get "Apps"() { return IrisStyle.identity.pink },
        get "At a glance"() { return IrisStyle.identity.sky },
        get "At rest"() { return IrisStyle.identity.indigo },
        get "Badges"() { return IrisStyle.identity.red },
        get "Banners"() { return IrisStyle.identity.red },
        get "Bar"() { return IrisStyle.identity.indigo },
        get "Behaviour"() { return IrisStyle.identity.green },
        get "Behind windows"() { return IrisStyle.identity.indigo },
        get "Borders"() { return IrisStyle.identity.orange },
        get "Bubble"() { return IrisStyle.identity.sky },
        get "Calendar"() { return IrisStyle.identity.red },
        get "Card contents"() { return IrisStyle.identity.teal },
        get "Cards"() { return IrisStyle.identity.blue },
        get "Charge limit"() { return IrisStyle.identity.green },
        get "Clipboard"() { return IrisStyle.identity.teal },
        get "Clock"() { return IrisStyle.identity.orange },
        get "Closing"() { return IrisStyle.identity.red },
        get "Room for Shima"() { return IrisStyle.identity.teal },
        get "Colour layer"() { return IrisStyle.identity.pink },
        get "Comfort"() { return IrisStyle.identity.yellow },
        get "Continue"() { return IrisStyle.identity.green },
        get "Control Center"() { return IrisStyle.identity.gray },
        get "Curve"() { return IrisStyle.identity.teal },
        get "Date & time"() { return IrisStyle.identity.orange },
        get "Desktop page"() { return IrisStyle.identity.teal },
        get "Do Not Disturb"() { return IrisStyle.identity.indigo },
        get "Extra bubbles"() { return IrisStyle.identity.green },
        get "Faces"() { return IrisStyle.identity.gray },
        get "Feedback"() { return IrisStyle.identity.orange },
        get "Floating"() { return IrisStyle.identity.sky },
        get "Focus · left"() { return IrisStyle.identity.green },
        get "Frame"() { return IrisStyle.identity.gray },
        get "Fullscreen"() { return IrisStyle.identity.blue },
        get "Game mode"() { return IrisStyle.identity.purple },
        get "Glass"() { return IrisStyle.identity.sky },
        get "Edges"() { return IrisStyle.identity.sky },
        get "Highlight"() { return IrisStyle.identity.orange },
        get "Icons"() { return IrisStyle.identity.yellow },
        get "Interaction"() { return IrisStyle.identity.green },
        get "Connections"() { return IrisStyle.identity.blue },
        get "Japanese lookup"() { return IrisStyle.identity.purple },
        get "Joining"() { return IrisStyle.identity.teal },
        get "Keyboard"() { return IrisStyle.identity.gray },
        get "Keyboard shortcuts"() { return IrisStyle.identity.indigo },
        get "Language"() { return IrisStyle.identity.teal },
        get "Layout"() { return IrisStyle.identity.blue },
        get "Light"() { return IrisStyle.identity.yellow },
        get "Live wallpapers"() { return IrisStyle.identity.pink },
        get "Look"() { return IrisStyle.identity.purple },
        get "Material"() { return IrisStyle.identity.indigo },
        get "Material per surface"() { return IrisStyle.identity.indigo },
        get "Menus"() { return IrisStyle.identity.gray },
        get "Motion"() { return IrisStyle.identity.indigo },
        get "Mouse"() { return IrisStyle.identity.blue },
        get "Movement"() { return IrisStyle.identity.indigo },
        get "Night Light"() { return IrisStyle.identity.orange },
        get "Notifications"() { return IrisStyle.identity.red },
        get "Now playing"() { return IrisStyle.identity.pink },
        get "On the contour"() { return IrisStyle.identity.teal },
        get "Opening bodies"() { return IrisStyle.identity.blue },
        get "Overview backdrop"() { return IrisStyle.identity.indigo },
        get "Pages"() { return IrisStyle.identity.blue },
        get "Panel look"() { return IrisStyle.identity.green },
        get "Parallax"() { return IrisStyle.identity.teal },
        get "Per surface"() { return IrisStyle.identity.orange },
        get "Performance"() { return IrisStyle.identity.green },
        get "Placement"() { return IrisStyle.identity.red },
        get "Player"() { return IrisStyle.identity.pink },
        get "Player page"() { return IrisStyle.identity.pink },
        get "Pointer"() { return IrisStyle.identity.orange },
        get "Previews"() { return IrisStyle.identity.teal },
        get "Read text"() { return IrisStyle.identity.blue },
        get "Recording"() { return IrisStyle.identity.pink },
        get "Resting Island"() { return IrisStyle.identity.blue },
        get "Scene"() { return IrisStyle.identity.teal },
        get "Screen"() { return IrisStyle.identity.blue },
        get "Security"() { return IrisStyle.identity.gray },
        get "Settings"() { return IrisStyle.identity.gray },
        get "Shape"() { return IrisStyle.identity.purple },
        get "Sign in"() { return IrisStyle.identity.blue },
        get "Size"() { return IrisStyle.identity.orange },
        get "Snip"() { return IrisStyle.identity.orange },
        get "Sound"() { return IrisStyle.identity.pink },
        get "Spotlight"() { return IrisStyle.identity.blue },
        get "Status"() { return IrisStyle.identity.green },
        get "Style per surface"() { return IrisStyle.identity.purple },
        get "Suspend"() { return IrisStyle.identity.indigo },
        get "Text"() { return IrisStyle.identity.gray },
        get "Timing"() { return IrisStyle.identity.orange },
        get "Today · right"() { return IrisStyle.identity.orange },
        get "Touch"() { return IrisStyle.identity.green },
        get "Trackpad"() { return IrisStyle.identity.teal },
        get "Tray"() { return IrisStyle.identity.teal },
        get "Type"() { return IrisStyle.identity.gray },
        get "Updates"() { return IrisStyle.identity.green },
        get "VPN"() { return IrisStyle.identity.blue },
        get "Visibility"() { return IrisStyle.identity.blue },
        get "Volume"() { return IrisStyle.identity.pink },
        get "Wallpaper"() { return IrisStyle.identity.teal },
        get "Wallpaper gallery"() { return IrisStyle.identity.purple },
        get "Warnings"() { return IrisStyle.identity.red },
        get "Weather"() { return IrisStyle.identity.sky },
        get "Web search"() { return IrisStyle.identity.teal },
        get "When idle"() { return IrisStyle.identity.indigo },
        get "Widgets"() { return IrisStyle.identity.teal },
        get "Windows"() { return IrisStyle.identity.blue },
        get "Workspaces"() { return IrisStyle.identity.indigo }
    })
    function sectionOf(spec: var): string {
        const path = String(spec.path ?? "")
        const group = String(spec.group ?? "")
        switch (spec.target) {
        case "island": return "bar"
        case "pieces": return "bubbles"
        case "bodies": return path.startsWith("iris.player.") ? "player" : group === "Control Center" ? "controlCenter" : "bubbles"
        case "places":
            if (path.startsWith("iris.wallpaper.") || path.includes(".gallery.")) return "desktop"
            if (group === "Spotlight") return "spotlight"
            if (group === "Panel look") return "sidebars"
            return "appearance"
        case "transients": return group === "Notifications" || group === "Banners" ? "notifications" : "sound"
        case "motion": return "motion"
        case "dock": return "dock"
        case "desktop": return "desktop"
        default: return "appearance"
        }
    }
    // Spotlight's "/" actions come from the rows themselves, so nothing is listed twice: every switch
    // Settings shows flips in place, the widget design and accents pick a value, and themes apply.
    // What "/" lists first with nothing typed: the switches people flip during a day.
    readonly property var quickFirst: ["iris.bar.notch", "iris.widgets.legibleAlways", "notifications.silent", "light.night.enabled",
        "performance.lowPower", "iris.bar.autoHide", "iris.surround.enable", "iris.appearance.anime.enabled", "iris.appearance.motion"]
    function quickActions(): var {
        const out = []
        for (const spec of root.settings) {
            if (!spec.path || !root.shown(spec)) continue
            const section = root.sectionById(String(spec.section ?? ""))
            const place = Translation.tr(section.title) + (spec.group ? " › " + Translation.tr(String(spec.group)) : "")
            const words = [spec.label, spec.group ?? "", section.title].concat(spec.keywords ?? []).join(" ")
            const icon = root.groupGlyphs[String(spec.group ?? "")] ?? section.icon
            if (spec.kind === "switch") {
                const first = root.quickFirst.indexOf(spec.path)
                out.push({ id: "set:" + spec.path, name: Translation.tr(spec.label), english: spec.label, detail: place, icon: icon, tint: section.tint,
                    area: section.id, areaName: Translation.tr(section.title),
                    words: words, priority: first >= 0 ? first : 100, isOn: () => spec.invert ? !Boolean(root.currentValue(spec)) : Boolean(root.currentValue(spec)),
                    run: () => root.commit(spec, !Boolean(root.currentValue(spec))) })
            } else if (spec.kind === "choice" && (spec.widgetDesign || spec.path === "iris.widgets.tint")) {
                for (const choice of root.choicesOf(spec))
                    out.push({ id: "set:" + spec.path + "=" + choice.value, name: Translation.tr(spec.quickName ?? spec.label) + ": " + Translation.tr(choice.label),
                        english: (spec.quickName ?? spec.label) + " " + choice.label, area: section.id, areaName: Translation.tr(section.title), detail: place, icon: String(choice.glyph ?? icon), tint: section.tint, words: words + " " + choice.label, priority: 50, pick: true,
                        isOn: () => root.currentValue(spec) === choice.value, run: () => root.commit(spec, choice.value) })
            }
        }
        for (const theme of IrisThemes.all)
            out.push({ id: "theme:" + theme.id, name: Translation.tr("Theme: %1").arg(theme.name), english: "Theme " + theme.name, detail: Translation.tr("Theme"), pick: true,
                area: "appearance", areaName: Translation.tr("Appearance"),
                icon: "palette", get tint() { return IrisStyle.identity.purple }, words: ["theme", "look", theme.name].concat(theme.tags ?? []).join(" "), priority: 60,
                isOn: () => IrisThemes.activeId === theme.id && !IrisThemes.modified, run: () => IrisThemes.choose(theme) })
        return out
    }
    // The order a section's groups are listed in: what someone came to change first, the rest as they were declared.
    readonly property var groupOrder: ({
        appearance: ["Look", "Themes", "Colour theme", "Scheme", "Dark look", "Ink look", "Light look", "Material", "Adaptive", "Glass", "Edges", "Shape", "Icons", "Corners per surface", "Frame", "Accent", "Highlight", "Colour layer", "Light", "Badges", "Wallpaper",
            "Text", "Faces", "Settings", "Menus", "Material per surface", "Customize", "Previews", "App colours"],
        bar: ["Notch", "Layout", "Shape", "At rest", "Resting Island", "Bar", "Pages", "Desktop page", "Player page", "Interaction", "Visibility", "Connections"],
        bubbles: ["Size", "Behaviour", "On the contour", "Floating", "Opening bodies", "Cards", "Card contents", "Joining", "Tray"],
        dock: ["Notch", "Look", "Icons", "Visibility"],
        desktop: ["Widgets", "Wallpaper shuffle", "Live wallpapers", "Behind windows", "Parallax", "Overview backdrop", "Wallpaper gallery", "Desktop menu"],
        lock: ["When idle", "Security", "Scene", "Clock", "At a glance", "Now playing", "Activity", "Status", "Sign in", "Type"]
    })
    readonly property var settings: {
        const rows = root.behaviour.concat(root.shared)
        const known = new Set(rows.map(spec => spec.path))
        const look = []
        for (const spec of root.studio) {
            if (spec.mirror) { look.push(Object.assign({ section: root.sectionOf(spec) }, spec)); continue }
            if (spec.path !== undefined && known.has(spec.path)) continue
            if (spec.path !== undefined) known.add(spec.path)
            look.push(Object.assign({ section: root.sectionOf(spec) }, spec))
        }
        const out = []
        for (const section of root.sectionOrder) {
            const own = rows.filter(spec => spec.section === section).concat(look.filter(spec => spec.section === section))
            const order = root.groupOrder[section] ?? []
            const rank = spec => { const at = order.indexOf(String(spec.group ?? "")); return at < 0 ? order.length : at }
            // Inside a group, what a surface does comes before how it looks (its corners, light and material).
            const lookRank = spec => String(spec.path ?? "").startsWith("iris.appearance.surfaces.") ? 1 : 0
            own.map((spec, at) => ({ spec: spec, at: at })).sort((a, b) => rank(a.spec) - rank(b.spec) || lookRank(a.spec) - lookRank(b.spec) || a.at - b.at).forEach(item => out.push(item.spec))
        }
        return out.map((spec, index) => Object.assign({}, spec, { modelKey: "settings:" + index }))
    }
}
