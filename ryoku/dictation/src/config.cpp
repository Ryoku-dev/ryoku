#include "config.hpp"

#include <algorithm>
#include <cctype>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <unistd.h>

namespace ryoku {
namespace {

std::string env(const char* key, const std::string& fallback) {
    const char* value = std::getenv(key);
    return value && *value ? std::string(value) : fallback;
}

std::string trim(const std::string& text) {
    const auto first = text.find_first_not_of(" \t\r\n");
    if (first == std::string::npos)
        return {};
    const auto last = text.find_last_not_of(" \t\r\n");
    return text.substr(first, last - first + 1);
}

int bounded(const std::string& text, int low, int high, int fallback) {
    char* end = nullptr;
    const long value = std::strtol(text.c_str(), &end, 10);
    if (end == text.c_str() || value < low || value > high)
        return fallback;
    return static_cast<int>(value);
}

bool truthy(const std::string& text) {
    return text == "true" || text == "1" || text == "yes" || text == "on";
}

} // namespace

const char* Config::sourceKey() { return "source"; }

std::string Config::path() {
    return env("XDG_CONFIG_HOME", env("HOME", ".") + "/.config") + "/ryoku/dictation.conf";
}

std::string Config::systemModel() {
    return env("RYOKU_DICTATION_PREFIX", "")
        + "/usr/share/ryoku/dictation/models/parakeet-tdt_ctc-110m-Q8_0.gguf";
}

std::string Config::userModelDir() {
    return env("XDG_DATA_HOME", env("HOME", ".") + "/.local/share") + "/ryoku/dictation/models";
}

std::string Config::socketPath() {
    const std::string runtime = env("XDG_RUNTIME_DIR", "");
    if (!runtime.empty())
        return runtime + "/ryoku-dictate.sock";
    return "/tmp/ryoku-dictate-" + std::to_string(::getuid()) + ".sock";
}

Config Config::load() {
    Config config;
    std::ifstream file(path());
    std::string line;
    while (std::getline(file, line)) {
        const auto equals = line.find('=');
        if (equals == std::string::npos || line[0] == '#')
            continue;
        const std::string key = trim(line.substr(0, equals));
        const std::string value = trim(line.substr(equals + 1));
        if (key == "enabled")
            config.enabled = truthy(value);
        else if (key == "model")
            config.model = value;
        else if (key == "device" && (value == "auto" || value == "cpu" || value == "gpu"))
            config.device = value;
        else if (key == "source")
            config.source = value == "default" ? std::string() : value;
        else if (key == "language" && value == "auto")
            config.language.clear();
        else if (key == "language")
            config.language = value;
        else if (key == "mode" && (value == "paste" || value == "type"))
            config.mode = value;
        else if (key == "max_duration_secs")
            config.maxDurationSecs = bounded(value, 10, 1800, config.maxDurationSecs);
        else if (key == "silence_ms")
            config.silenceMs = bounded(value, 0, 10000, config.silenceMs);
        else if (key == "silence_db")
            config.silenceDb = bounded(value, -80, -10, config.silenceDb);
        else if (key == "type_delay_ms")
            config.typeDelayMs = bounded(value, 0, 100, config.typeDelayMs);
        else if (key == "idle_unload_secs")
            config.idleUnloadSecs = bounded(value, 0, 3600, config.idleUnloadSecs);
    }
    return config;
}

std::string Config::resolveModel(const Config& config) {
    if (!config.model.empty())
        return std::filesystem::exists(config.model) ? config.model : std::string();

    const std::string system = systemModel();
    const std::string name = std::filesystem::path(system).filename().string();
    const std::string user = userModelDir() + "/" + name;
    if (std::filesystem::exists(user))
        return user;
    if (std::filesystem::exists(system))
        return system;

    std::string newest;
    std::filesystem::file_time_type newestAt{};
    std::error_code ec;
    for (const auto& entry : std::filesystem::directory_iterator(userModelDir(), ec)) {
        if (!entry.is_regular_file() || entry.path().extension() != ".gguf")
            continue;
        const auto at = entry.last_write_time(ec);
        if (newest.empty() || at > newestAt) {
            newest = entry.path().string();
            newestAt = at;
        }
    }
    return newest;
}

} // namespace ryoku
