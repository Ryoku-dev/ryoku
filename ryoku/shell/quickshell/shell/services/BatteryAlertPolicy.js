// The live QML service and regression tests use this same policy.
function initialState() { return { lowSent: false, criticalSent: false }; }
function evaluate(state, sample, settings) {
    var next = { lowSent: state.lowSent, criticalSent: state.criticalSent };
    if (sample.onAc) return { state: initialState(), level: "" };
    if (!settings.enabled || !sample.present || !sample.ready || !sample.discharging
            || !Number.isFinite(sample.percent) || sample.percent < 0 || sample.percent > 100)
        return { state: next, level: "" };
    if (sample.percent <= settings.critical && !next.criticalSent) {
        next.lowSent = true; next.criticalSent = true;
        return { state: next, level: "critical" };
    }
    if (sample.percent <= settings.low && !next.lowSent) {
        next.lowSent = true;
        return { state: next, level: "low" };
    }
    return { state: next, level: "" };
}
