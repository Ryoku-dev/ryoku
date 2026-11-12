.pragma library

// the install or update running now, kept past the page that started it so a page
// opened again shows it busy: { id, verb, proc, started } or null
var current = null;

function start(run) {
    current = run;
}

// called from the install's own callback, which outlives the page that made it,
// so it holds this function rather than the page's import of the library
function finish(run) {
    if (current === run)
        current = null;
}
