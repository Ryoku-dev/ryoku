.pragma library

// wmo weather code to a material symbols name
function symbol(code, day) {
    if (code < 0) return "cloud";
    if (code === 0) return day ? "clear_day" : "clear_night";
    if (code === 1 || code === 2) return day ? "partly_cloudy_day" : "partly_cloudy_night";
    if (code === 45 || code === 48) return "foggy";
    if (code >= 95) return "thunderstorm";
    if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "weather_snowy";
    if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82)) return "rainy";
    return "cloud";
}
