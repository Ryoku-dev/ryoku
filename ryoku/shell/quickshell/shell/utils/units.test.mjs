import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { bytes, rate, gib, pct, uptime, hz, tempUnit, temp } = require("./units.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}

eq(bytes(0), "0 B", "zero bytes");
eq(bytes(999), "999 B", "bytes under a kilo");
eq(bytes(1500), "1.5 kB", "kilobytes");
eq(bytes(2400000), "2.4 MB", "megabytes");
eq(bytes(2400000000), "2.4 GB", "gigabytes");
eq(rate(0), "0", "idle network reads as zero");
eq(rate(1250000), "1.3 MB/s", "rate adds a per second suffix");
eq(gib(15.6), "15.6", "one decimal of gibibytes");
eq(pct(0.42), "42%", "fraction to percent");
eq(uptime(90), "1m", "under an hour");
eq(uptime(3 * 3600 + 12 * 60), "3h 12m", "hours and minutes");
eq(uptime(2 * 86400 + 5 * 3600), "2d 5h", "days and hours");
eq(hz("1920x1080@200.000"), 200, "refresh rate out of a mode string");
eq(hz("2560x1440@59.951"), 60, "rounds a fractional refresh");
eq(hz(""), 0, "no mode, no rate");
eq(tempUnit("celsius", "en_US.UTF-8"), "celsius", "an explicit unit wins over the locale");
eq(tempUnit("fahrenheit", "de_DE.UTF-8"), "fahrenheit", "an explicit Fahrenheit wins too");
eq(tempUnit("auto", "en_US.UTF-8"), "fahrenheit", "auto follows a US locale");
eq(tempUnit("auto", "de_DE.UTF-8"), "celsius", "auto follows a metric locale");
eq(tempUnit("auto", ""), "celsius", "no locale falls back to Celsius");
eq(temp(21.6, "celsius"), "22 °C", "a Celsius reading rounds to the degree");
eq(temp(21.6, "fahrenheit"), "71 °F", "a Fahrenheit reading converts and rounds");
eq(temp(0, "fahrenheit"), "32 °F", "freezing in Fahrenheit");
eq(temp(NaN, "celsius"), "n/a", "a missing reading is not a temperature");

if (failed > 0) {
    console.log(failed + " failure(s)");
    process.exit(1);
}
console.log("all units tests passed");
