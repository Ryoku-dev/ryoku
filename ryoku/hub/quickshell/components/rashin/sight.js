.pragma library

var languages = [
    { code: "eng", label: "English" },
    { code: "jpn", label: "Japanese" },
    { code: "kor", label: "Korean" },
    { code: "chi_sim", label: "Chinese (Simplified)" },
    { code: "chi_tra", label: "Chinese (Traditional)" },
    { code: "deu", label: "German" },
    { code: "fra", label: "French" },
    { code: "spa", label: "Spanish" },
    { code: "ita", label: "Italian" },
    { code: "por", label: "Portuguese" },
    { code: "rus", label: "Russian" },
    { code: "ara", label: "Arabic" },
    { code: "hin", label: "Hindi" },
    { code: "vie", label: "Vietnamese" },
    { code: "tha", label: "Thai" },
    { code: "tur", label: "Turkish" },
    { code: "pol", label: "Polish" },
    { code: "nld", label: "Dutch" },
    { code: "ukr", label: "Ukrainian" }
];

function targetOptions(langs, tr) {
    return [{ label: tr("Your language"), value: "" }].concat(langs.map(l => {
        var label = tr(l.name);
        return { label: label + (l.native && l.native !== label ? " · " + l.native : ""), value: l.code };
    }));
}

function targetCode(value, langs) {
    var code = value === "zh-Hans" ? "zh_CN" : value === "zh-Hant" ? "zh_TW" : value;
    return langs.some(l => l.code === code) ? code : "";
}

function missingLanguages(selected, installed) {
    return selected.filter((code, index) => selected.indexOf(code) === index
        && installed.indexOf(code) < 0 && languages.some(l => l.code === code));
}

function modelSuggestions(text) {
    var seen = Object.create(null);
    return String(text || "").split("\n").map(line => line.split("\t")[0].trim())
        .filter(id => {
            if (!id || seen[id])
                return false;
            seen[id] = true;
            return true;
        }).map(id => ({ id: id, name: id }));
}
