.pragma library

// Application windows on the workspace an output is showing, from the Wm
// seam's rows. Those rows are already application toplevels; the
// foreign-toplevel handle is enrichment that can briefly be missing while a
// title changes, so a row without one still counts and the frost holds steady.
function countOnActiveWorkspace(windows, workspaces, outputName) {
    const output = String(outputName || "");
    const active = [];
    const spaces = workspaces || [];
    for (let i = 0; i < spaces.length; i++) {
        const workspace = spaces[i];
        if (!workspace || workspace.active !== true || workspace.special === true)
            continue;
        if (output !== "" && workspace.output !== output)
            continue;
        active.push(String(workspace.id));
    }
    if (active.length === 0)
        return 0;

    let count = 0;
    const rows = windows || [];
    for (let j = 0; j < rows.length; j++) {
        const window = rows[j];
        if (!window || active.indexOf(String(window.workspace)) < 0)
            continue;
        if (window.toplevel && window.toplevel.minimized === true)
            continue;
        count++;
    }
    return count;
}
