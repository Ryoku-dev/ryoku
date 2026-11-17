// Short date line in the reader's own order: "Tue, Nov 17" where the locale
// puts the month first, "mar, 17 nov" where it does not.
function dateLine(loc, date, longDay) {
    var day = longDay ? "dddd" : "ddd";
    var short = loc.dateFormat(1);
    var month = short.indexOf("M");
    var monthFirst = month >= 0 && month < short.indexOf("d");
    return loc.toString(date, monthFirst ? day + ", MMM d" : day + ", d MMM");
}
