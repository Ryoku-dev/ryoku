function empty() {
    return { states: null, finished: 0, failed: 0 };
}

function seen(activity) {
    return { states: activity.states, finished: 0, failed: 0 };
}

function update(activity, tasks, current) {
    var states = {};
    var finished = current ? 0 : activity.finished;
    var failed = current ? 0 : activity.failed;
    for (var i = 0; i < tasks.length; i++) {
        var id = String(tasks[i].id);
        var state = String(tasks[i].state);
        states[id] = state;
        if (current || activity.states === null || activity.states[id] === state)
            continue;
        if (state === "done" || state === "seeding") {
            if (activity.states[id] !== "done" && activity.states[id] !== "seeding")
                finished++;
        } else if (state === "error") {
            failed++;
        }
    }
    return { states: states, finished: finished, failed: failed };
}

if (typeof module !== "undefined")
    module.exports = { empty, seen, update };
