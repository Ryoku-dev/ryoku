// Window-local recognition for Ryoku Hub's quiet keepsake.
var CODE = "WWSSADAD";
var GAP_MS = 2400;

function step(index, lastAt, key, at, modified, repeated) {
    if (modified || repeated || CODE.indexOf(key) < 0)
        return { index: 0, at: 0, complete: false };

    var current = (lastAt === 0 || at - lastAt > GAP_MS || at < lastAt) ? 0 : index;
    current = key === CODE.charAt(current) ? current + 1 : (key === CODE.charAt(0) ? 1 : 0);
    return { index: current === CODE.length ? 0 : current, at: at,
             complete: current === CODE.length };
}
