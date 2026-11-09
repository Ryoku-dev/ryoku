// The Game Bar's recording lifecycle. Kept apart from the visibility hold:
// reopening the bar during a recording releases the hold, but the cycle and the
// pre-recording strip state survive until the recording ends, so the bar can
// return to how it was.

function begin(open) {
    return {
        reopen: Boolean(open),
        closeBar: Boolean(open)
    };
}

function release(hold, cycle) {
    return Boolean(hold && cycle);
}

function finish(cycle, reopen, open) {
    if (!cycle)
        return null;
    return {
        cycle: false,
        hold: false,
        openBar: Boolean(reopen) && !open,
        closeBar: !reopen && Boolean(open)
    };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { begin, release, finish };
