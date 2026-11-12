// An action may carry a placeholder tail like RemoveResolutionPreset(idx); the id is the part before it.

function idOf(action) {
    if (typeof action !== "string")
        return "";
    var i = action.indexOf("(");
    return i < 0 ? action : action.substring(0, i);
}

function isDestructive(action) {
    return /^(Delete|Remove|Clear|Reset)/.test(idOf(action));
}

function isImmediate(action) {
    var id = idOf(action);
    return id.indexOf("Open") === 0 || id.indexOf("Add") === 0 || id.indexOf("Create") === 0
        || id === "RefreshAppThemes" || id === "RefreshBackdrop" || id === "ChooseRunningProcess"
        || id === "ImportSemanticModel";
}

function _band(band) {
    var map = {
        fhd: { label: "FHD", w: 1920, h: 1080 },
        qhd: { label: "2K", w: 2560, h: 1440 },
        "4k": { label: "4K", w: 3840, h: 2160 },
        "5k": { label: "5K", w: 5120, h: 2880 },
        "8k": { label: "8K", w: 7680, h: 4320 },
        custom: { label: "Custom", w: 0, h: 0 }
    };
    var b = map[band] || map.custom;
    return {
        label: b.label, orientation: "",
        minWidth: b.w, minHeight: b.h, maxWidth: 0, maxHeight: 0
    };
}

// list settings reach QML as Qt sequences, which Array.isArray rejects
function asList(v) {
    return v !== null && typeof v === "object" && typeof v.length === "number" ? Array.prototype.slice.call(v) : [];
}

function _list(settings, key) {
    return asList(settings.value(key));
}

function _push(settings, key, element) {
    var arr = _list(settings, key);
    arr.push(element);
    settings.set(key, arr);
}

function _removeAt(settings, key, index) {
    var arr = _list(settings, key);
    if (index >= 0 && index < arr.length) {
        arr.splice(index, 1);
        settings.set(key, arr);
    }
}

// Lists persist whole: the daemon has no indexed sub-keys.
function setField(settings, key, index, field, value) {
    var arr = _list(settings, key);
    if (index < 0 || index >= arr.length)
        return;
    var el = arr[index] || ({});
    var copy = ({});
    for (var k in el) if (el.hasOwnProperty(k)) copy[k] = el[k];
    copy[field] = value;
    arr[index] = copy;
    settings.set(key, arr);
}

function setMapField(settings, key, field, value) {
    var v = settings.value(key);
    var m = (v && typeof v === "object" && !Array.isArray(v)) ? v : ({});
    var copy = ({});
    for (var k in m) if (m.hasOwnProperty(k)) copy[k] = m[k];
    copy[field] = value;
    settings.set(key, copy);
}

function run(action, args, settings, state, host) {
    var id = idOf(action);
    args = args || ({});
    switch (id) {
    case "OpenScheduleEditor": state.openSheet("schedule", ({})); return;
    case "OpenThemeDesigner": state.openSheet("themeDesigner", ({})); return;
    case "OpenTimeLanguage": state.runAction(id, args); state.hideRequested(); return;
    case "CreateResolutionPresetBand": _push(settings, "filterBar.resolutionPresets", _band(args.band)); return;
    case "RemoveResolutionPreset": _removeAt(settings, "filterBar.resolutionPresets", args.index); return;
    case "AddPostCommand": _push(settings, "postProcessing", { type: "all", command: "" }); return;
    case "RemovePostCommand": _removeAt(settings, "postProcessing", args.index); return;
    case "AddIntegration": _push(settings, "integrations",
        { enabled: true, name: "", template: "", output: "", reload: "", livePreview: false }); return;
    case "RemoveIntegration": _removeAt(settings, "integrations", args.index); return;
    case "ResetKeybinds":
        if (Array.isArray(args.keys))
            for (var i = 0; i < args.keys.length; i++) settings.reset(args.keys[i]);
        return;
    case "ChooseRunningProcess":
        if (host && host.chooseProcess) host.chooseProcess(args.onChosen);
        return;
    default:
        state.runAction(id, args);
    }
}
