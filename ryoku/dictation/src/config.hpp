#pragma once

#include <string>

namespace ryoku {

// The dictation settings, read from ~/.config/ryoku/dictation.conf.
//
// A flat key=value file, like keysounds: the daemon is the only writer of the
// meaning, the Hub edits the same keys, and a missing file simply means the
// defaults below. Nothing here is a compositor or shell concept.
struct Config {
    bool enabled = false;
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

    static std::string systemModel();
    static std::string userModelDir();
    static std::string resolveModel(const Config& config);

    static const char* sourceKey();
    static std::string socketPath();
};

} // namespace ryoku
