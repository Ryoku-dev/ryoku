.pragma library

function activeWorkspace(workspaces, screenName, isolateMonitors, focusedWorkspace) {
    if (!isolateMonitors || !screenName) return focusedWorkspace
    for (var i = 0; i < workspaces.length; i++) {
        var workspace = workspaces[i]
        if (!workspace.special && workspace.active && workspace.output === screenName)
            return workspace
    }
    return null
}

// some compositors number workspaces per output, so a name alone can match
// another screen's workspace
function workspaceOn(workspaces, name, screenName, isolateMonitors) {
    var isolated = isolateMonitors && screenName
    for (var i = 0; i < workspaces.length; i++) {
        var workspace = workspaces[i]
        if (workspace.special || workspace.name !== name) continue
        if (!isolated || !workspace.output || workspace.output === screenName)
            return workspace
    }
    return null
}

function needsWorkspaceData(mode, dynamicModel, isolateMonitors) {
    return dynamicModel || mode === "active" || isolateMonitors
}

function workspaceList(workspaces, mode, dynamicModel, screenName, isolateMonitors, active, extraWorkspace) {
    if (dynamicModel) {
        var live = []
        for (var i = 0; i < workspaces.length; i++) {
            var workspace = workspaces[i]
            if (workspace.special) continue
            if (isolateMonitors && screenName && workspace.output && workspace.output !== screenName)
                continue
            live.push(workspace.name)
        }
        return live
    }

    if (mode === "active") {
        var ids = {}
        for (var j = 0; j < workspaces.length; j++) {
            var current = workspaces[j]
            if (current.special) continue
            if (isolateMonitors && screenName && current.output && current.output !== screenName)
                continue
            ids[current.name] = true
        }
        if (active && !active.special
                && (!isolateMonitors || !screenName || !active.output || active.output === screenName))
            ids[active.name] = true
        return Object.keys(ids).sort(function(a, b) { return Number(a) - Number(b) })
    }

    var count = mode === "5" ? 5 : 10
    var fixed = []
    for (var k = 1; k <= count; k++) {
        var name = String(k)
        var existing = workspaces.find(function(workspace) { return workspace.name === name })
        if (isolateMonitors && screenName && existing && existing.output && existing.output !== screenName)
            continue
        fixed.push(name)
    }
    if (extraWorkspace) fixed.push(extraWorkspace)
    return fixed
}

function extraWorkspace(focused, mode) {
    var count = mode === "5" ? 5 : 10
    var number = focused ? Number(focused.name) : 0
    return number > count ? focused.name : ""
}

function hasMonitorFocus(isolateMonitors, screenName, focusedOutput) {
    return !isolateMonitors || !screenName || !focusedOutput || focusedOutput === screenName
}

function markerOpacity(hasFocus) {
    return hasFocus ? 1.0 : 0.72
}
