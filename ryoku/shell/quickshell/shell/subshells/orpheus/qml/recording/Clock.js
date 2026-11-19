.pragma library

function format(total) {
    var whole = Math.max(0, Math.floor(Number(total) || 0));
    var hours = Math.floor(whole / 3600);
    var minutes = Math.floor((whole % 3600) / 60);
    var rest = whole % 60;
    var two = function(value) { return (value < 10 ? "0" : "") + value; };
    return hours > 0 ? hours + ":" + two(minutes) + ":" + two(rest)
        : two(minutes) + ":" + two(rest);
}
