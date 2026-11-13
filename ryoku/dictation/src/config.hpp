#pragma once

#include <string>

namespace ryoku {

// The dictation settings, read from ~/.config/ryoku/dictation.conf.
//
// A flat key=value file, like keysounds: the daemon is the only writer of the
// meaning, the Hub edits the same keys, and a missing file simply means the
// defaults below. Nothing here is a compositor or shell concept.
struct Config {
    bool enabled = true;
    // Empty picks the shipped model; an explicit path wins over it.
    std::string model;
    // auto | cpu | gpu. gpu refuses to fall back silently: a user who asked for
    // the GPU must hear that the backend is missing, not feel a slow CPU.
    std::string device = "auto";
    // Microphone node to record from. Empty is the session default, which is
    // what almost everyone wants; naming one pins dictation to that device.
    std::string source;
    // Empty lets the model detect the language (Parakeet v2 is English-only and
    // ignores it; v3/ultra use it as a hint).
    std::string language;
    // paste (clipboard + Ctrl+V) or type (synthesised keystrokes).
    std::string mode = "paste";
    int maxDurationSecs = 120;
    // Stop on its own after this much silence, once something was said.
    // 0 keeps recording until the key is tapped again.
    int silenceMs = 1500;
    int silenceDb = -45;
    int typeDelayMs = 2;
    // Unmap the model after the daemon has been idle this long.
    int idleUnloadSecs = 90;

    static Config load();
    static std::string path();

    // /usr/share/ryoku/dictation/models/... — the model the package ships.
    // RYOKU_DICTATION_PREFIX relocates the whole read-only tree, so a package
    // staging directory can be proved without installing it.
    static std::string shippedModel();
    static std::string userModelDir();
    // explicit path, then the user copy of the shipped name, then the shipped
    // file, then the newest .gguf in the user directory. Empty when none exists.
    static std::string resolveModel(const Config& config);

    static const char* sourceKey();
    static std::string socketPath();
};

} // namespace ryoku
