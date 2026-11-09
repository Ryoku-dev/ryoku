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

function workspaceList(workspaces, mode, dynamicModel, screenName, isolateMonitors, active) {
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
    if (active) {
        var activeNumber = Number(active.name)
        if (activeNumber > count) fixed.push(active.name)
    }
    return fixed
}

function hasMonitorFocus(isolateMonitors, screenName, focusedOutput) {
    return !isolateMonitors || !screenName || !focusedOutput || focusedOutput === screenName
}

function markerOpacity(hasFocus) {
    return hasFocus ? 1.0 : 0.72
}
