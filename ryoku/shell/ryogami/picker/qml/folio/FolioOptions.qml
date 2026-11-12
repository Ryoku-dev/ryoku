import QtQuick
import Ryoku.Ui.Singletons
import "FolioActions.js" as Actions

// A control names a provider in its dynamic field; optionsFor() merges base options with the live entries.
Item {
    id: opts

    // Bumped whenever any provider's source changes, so option bindings re-run.
    property int revision: 0

    // Folder enumeration is lazy and cached by library generation. The cache lives in a
    // plain object: folders() runs inside option bindings, and writing a notifying
    // property there would re-trigger the binding that is reading it.
    readonly property var _folderMemo: ({ gen: -1, list: [] })

    Connections {
        target: Library
        function onOutputsChanged() { opts.revision++ }
        function onChanged(collection) { opts.revision++ }
    }
    Connections {
        target: Settings
        function onChanged(key, value) {
            if (key === "semantic.models" || key === "transition.family")
                opts.revision++
        }
        function onSchemaChanged() { opts.revision++ }
    }
    Connections {
        target: I18n
        function onLangsChanged() { opts.revision++ }
    }

    LibraryView {
        id: folderScan
        collection: "wallpapers"
    }

    // Families mirror the renderer's shader grouping; keep them in step when it adds shaders.
    readonly property var _shaderFamilies: ({
        fade: ["crossfade", "fadecolor", "static-fade", "colour-distance", "chromatic-bloom",
               "overexposure", "soft-warp-fade", "smoke", "ink-splash", "inkwell-drop", "plasma-flow"],
        wipe: ["directional", "directional-wipe", "directional-scaled", "edge-transition", "circle-crop",
               "iris", "polka-dots-curtain", "crosshatch", "polar-function", "puzzle-right"],
        warp: ["wave-warp", "liquid-ripple", "crazy-parametric", "parametric-glitch", "zoom-blur-pull",
               "morph", "bounce", "perlin", "flyeye", "pixelfade-wave", "heat-melt", "crosswarp"],
        break: ["pixelate", "voronoi-shatter", "glitch", "glitch-displace", "mosaic-tumble", "randomsquares"],
        sand: ["sand"]
    })

    function _title(s) {
        var parts = String(s).split(/[-_]/);
        for (var i = 0; i < parts.length; i++)
            parts[i] = parts[i].length ? parts[i].charAt(0).toUpperCase() + parts[i].slice(1) : parts[i];
        return parts.join(" ");
    }

    // System default maps to the empty stored value.
    function languages() {
        var out = [{ value: "", label: I18n.tr("System default") }];
        var codes = I18n.pickerOptions;
        var labels = I18n.pickerLabels;
        for (var i = 0; i < codes.length; i++) {
            var code = codes[i];
            if (code === "auto")
                continue;
            out.push({ value: code, label: labels[code] !== undefined ? labels[code] : code });
        }
        return out;
    }

    function outputs() {
        var out = [];
        var list = Library.outputs;
        for (var i = 0; i < list.length; i++) {
            var o = list[i];
            out.push({ value: o.name, label: o.name });
        }
        return out;
    }

    function folders() {
        var memo = opts._folderMemo;
        if (folderScan.generation !== memo.gen) {
            var seen = ({});
            var acc = [];
            var n = folderScan.count;
            for (var r = 0; r < n; r++) {
                var e = folderScan.get(r);
                var f = e && e.folder ? String(e.folder) : "";
                if (f.length > 0 && !seen[f]) { seen[f] = true; acc.push(f); }
            }
            acc.sort();
            memo.list = acc;
            memo.gen = folderScan.generation;
        }
        var out = [];
        for (var i = 0; i < memo.list.length; i++)
            out.push({ value: memo.list[i], label: memo.list[i] });
        return out;
    }

    function semanticModels() {
        var out = [];
        var models = Actions.asList(Settings.value("semantic.models"));
        for (var i = 0; i < models.length; i++) {
            var m = models[i] || {};
            var val = m.manifest !== undefined ? m.manifest : (m.id !== undefined ? m.id : "");
            var lab = m.name !== undefined && m.name !== "" ? m.name : val;
            if (val !== "")
                out.push({ value: val, label: lab });
        }
        return out;
    }

    function transitionOptions() {
        var family = String(Settings.value("transition.family") || "random");
        var names = opts._shaderFamilies[family];
        if (!names || names.length === 0)
            names = opts._shaderFamilies.fade;
        var out = [];
        for (var i = 0; i < names.length; i++)
            out.push({ value: names[i], label: opts._title(names[i]) });
        return out;
    }

    function themes() {
        var out = [];
        var keys = Object.keys(Theme.presets);
        for (var i = 0; i < keys.length; i++)
            out.push({ value: keys[i], label: opts._title(keys[i]) });
        return out;
    }

    function recolourThemes() {
        return themes();
    }

    property Connections _gpuConn: Connections {
        target: GpuDevices
        function onDevicesChanged() { opts.revision++; }
    }
    function gpus() {
        var list = GpuDevices.devices;
        var out = [];
        var known = ({});
        for (var i = 0; i < list.length; i++) {
            var label = list[i].name;
            if (list[i].unavailable === "restart")
                label = I18n.tr("%1 (restart to use it)").arg(label);
            else if (list[i].unavailable)
                label = I18n.tr("%1 (driver not loaded)").arg(label);
            out.push({ value: list[i].id, label: label });
            known[list[i].id] = true;
        }
        var current = String(Settings.value("performance.gpuDevice") || "");
        if (current.length > 0 && current !== "auto" && !known[current])
            out.push({ value: current, label: I18n.tr("Unavailable GPU") });
        return out;
    }

    function _provide(name) {
        switch (name) {
        case "languages": return languages();
        case "outputs": return outputs();
        case "folders": return folders();
        case "semanticModels": return semanticModels();
        case "transitions": return transitionOptions();
        case "themes": return themes();
        case "recolourThemes": return recolourThemes();
        case "gpus": return gpus();
        }
        return [];
    }

    // Authored base options stay first so a chosen special entry keeps its place at the head.
    function optionsFor(control) {
        var base = control && control.options ? control.options : [];
        if (!control || !control.dynamic)
            return base;
        var out = [];
        var seen = ({});
        for (var i = 0; i < base.length; i++) {
            out.push(base[i]);
            seen[String(base[i].value)] = true;
        }
        var extra = _provide(control.dynamic);
        for (var j = 0; j < extra.length; j++) {
            if (!seen[String(extra[j].value)]) {
                out.push(extra[j]);
                seen[String(extra[j].value)] = true;
            }
        }
        return out;
    }
}
