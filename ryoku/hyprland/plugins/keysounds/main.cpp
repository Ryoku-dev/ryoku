// keysounds: a Hyprland plugin that plays a keyboard sound on every key press.
//
// The compositor sees every key before any client does, so one plugin covers
// every app with no per-app hook and no input grabbing. Samples come from a
// profile: a directory of short WAV/OGG files named by role (down-*, up-*,
// space-*, enter-*, backspace-*), a random variant per press so typing never
// sounds like a loop. Playback goes through libcanberra, the freedesktop event
// sound library: the server caches each sample after its first play, so a
// press costs one small request and lands with no perceptible lag, and
// overlapping presses mix in the sound server rather than here.
//
// Config (plugin:keysounds:*): enabled, profile, volume (0..1), release (also
// play the key-up sample). A profile is a directory of real switch recordings
// (the shipped ones are cut from Mechvibes packs; ryoku-keysounds-import makes
// one from any pack). Profiles are looked up under
// $XDG_DATA_HOME/ryoku/keysounds (the user's own), beside this .so
// (<dir>/keysounds, what a local build lays down), then
// /usr/share/ryoku/keysounds (the package).

#include <hyprland/src/plugins/PluginAPI.hpp>
#include <hyprland/src/config/values/types/BoolValue.hpp>
#include <hyprland/src/config/values/types/FloatValue.hpp>
#include <hyprland/src/config/values/types/StringValue.hpp>
#include <hyprland/src/devices/IKeyboard.hpp>
#include <hyprland/src/event/EventBus.hpp>

#include <canberra.h>
#include <dlfcn.h>
#include <linux/input-event-codes.h>

#include <algorithm>
#include <cmath>
#include <cstdlib>
#include <filesystem>
#include <map>
#include <random>
#include <string>
#include <vector>

inline HANDLE PHANDLE = nullptr;

namespace {
    namespace fs = std::filesystem;

    struct {
        SP<Config::Values::CBoolValue>   enabled;
        SP<Config::Values::CStringValue> profile;
        SP<Config::Values::CFloatValue>  volume;
        SP<Config::Values::CBoolValue>   release;
    } cfg;

    // the shipped default, what a profile that went away falls back to
    constexpr const char* DEFAULT_PROFILE = "cherry-mx-brown";

    // one profile, resolved: the sample files per role. A role is a filename
    // prefix (down-1.wav); a key with no samples of its own uses the generic
    // press or release.
    struct SBank {
        std::string                                     name, dir;
        std::map<std::string, std::vector<std::string>> roles{
            {"down", {}}, {"up", {}}, {"space", {}}, {"space-up", {}}, {"enter", {}}, {"enter-up", {}}, {"backspace", {}}, {"backspace-up", {}},
        };
    };

    ca_context*                                  ctx = nullptr;
    SBank                                        bank;
    std::mt19937                                 rng{std::random_device{}()};
    Hyprutils::Signal::CHyprSignalListener       keyListener;

    std::string envOr(const char* name, const std::string& fallback) {
        const char* v = std::getenv(name);
        return v && *v ? v : fallback;
    }

    // the directory this .so was loaded from, for the profiles a local build
    // lays beside it
    std::string ownDir() {
        Dl_info info{};
        if (dladdr(reinterpret_cast<void*>(&ownDir), &info) && info.dli_fname)
            return fs::path(info.dli_fname).parent_path().string();
        return "";
    }

    std::vector<std::string> profileRoots() {
        std::vector<std::string> roots;
        const auto home = envOr("HOME", "");
        roots.push_back(envOr("XDG_DATA_HOME", home + "/.local/share") + "/ryoku/keysounds");
        if (const auto d = ownDir(); !d.empty())
            roots.push_back(d + "/keysounds");
        roots.push_back("/usr/share/ryoku/keysounds");
        return roots;
    }

    bool isSample(const fs::path& p) {
        const auto e = p.extension().string();
        return e == ".wav" || e == ".oga" || e == ".ogg";
    }

    // files named <role>-<n>.<ext>: "down" must not sweep "down-up" style roles,
    // so the match is the role followed by its separator
    void collect(const fs::path& dir, const std::string& role, std::vector<std::string>& into) {
        std::error_code ec;
        const auto prefix = role + "-";
        for (const auto& e : fs::directory_iterator(dir, ec)) {
            if (!e.is_regular_file(ec) || !isSample(e.path()))
                continue;
            if (e.path().filename().string().rfind(prefix, 0) == 0)
                into.push_back(e.path().string());
        }
        std::sort(into.begin(), into.end());
    }

    // upload every sample to the sound server once, so the first press is as
    // quick as the hundredth
    void cacheAll(const std::vector<std::string>& files) {
        if (!ctx)
            return;
        for (const auto& f : files)
            ca_context_cache(ctx, CA_PROP_MEDIA_FILENAME, f.c_str(), CA_PROP_CANBERRA_CACHE_CONTROL, "permanent", nullptr);
    }

    // a profile directory by name across the roots, "" when none has it
    std::string findProfile(const std::string& name) {
        for (const auto& root : profileRoots()) {
            const fs::path dir = fs::path(root) / name;
            std::error_code ec;
            if (fs::is_directory(dir, ec))
                return dir.string();
        }
        return "";
    }

    void loadBank() {
        const std::string want = cfg.profile ? cfg.profile->value() : DEFAULT_PROFILE;
        SBank             next;
        next.name = want;
        next.dir  = findProfile(want);
        if (next.dir.empty() && want != DEFAULT_PROFILE) {
            // a profile that went away (renamed, a user folder removed): fall
            // back to the shipped default rather than go silent
            HyprlandAPI::addNotification(PHANDLE, "[keysounds] no profile named \"" + want + "\" under " + profileRoots().front() + " or /usr/share/ryoku/keysounds; using " + DEFAULT_PROFILE, CHyprColor{1.0, 0.6, 0.2, 1.0}, 6000);
            next.dir = findProfile(DEFAULT_PROFILE);
        }
        if (next.dir.empty()) {
            bank = std::move(next);
            HyprlandAPI::addNotification(PHANDLE, "[keysounds] no sample profiles installed (is ryoku-keysounds installed?)", CHyprColor{1.0, 0.6, 0.2, 1.0}, 6000);
            return;
        }
        for (auto& [role, files] : next.roles)
            collect(next.dir, role, files);
        bank = std::move(next);
        for (const auto& [role, files] : bank.roles)
            cacheAll(files);
    }

    // the samples for a key: the key's own role when the profile records it,
    // else the generic press or release
    const std::vector<std::string>& bucketFor(uint32_t keycode, bool released) {
        const char* role = nullptr;
        switch (keycode) {
            case KEY_SPACE: role = "space"; break;
            case KEY_ENTER:
            case KEY_KPENTER: role = "enter"; break;
            case KEY_BACKSPACE: role = "backspace"; break;
        }
        const auto& generic = bank.roles.at(released ? "up" : "down");
        if (!role)
            return generic;
        const auto& own = bank.roles.at(released ? std::string(role) + "-up" : role);
        return own.empty() ? generic : own;
    }

    void play(const std::vector<std::string>& files) {
        if (!ctx || files.empty())
            return;
        const float vol = cfg.volume ? std::clamp(static_cast<float>(cfg.volume->value()), 0.F, 1.F) : 0.6F;
        if (vol <= 0.F)
            return;
        const auto& file = files[std::uniform_int_distribution<size_t>(0, files.size() - 1)(rng)];
        // canberra takes the volume as a dB offset; 1.0 is the sample as recorded
        const std::string db = std::to_string(20.0 * std::log10(vol));
        ca_context_play(ctx, 0, CA_PROP_MEDIA_FILENAME, file.c_str(), CA_PROP_CANBERRA_VOLUME, db.c_str(), CA_PROP_CANBERRA_CACHE_CONTROL, "permanent", nullptr);
    }

    void onKey(const IKeyboard::SKeyEvent& e) {
        if (!cfg.enabled || !cfg.enabled->value())
            return;
        // the profile is read here, not on a reload event, so a change made
        // through hl.config at runtime (the Hub's Save, an eval) takes effect on
        // the next press without a full reload
        if (cfg.profile && cfg.profile->value() != bank.name)
            loadBank();
        if (bank.dir.empty())
            return;
        if (e.state == WL_KEYBOARD_KEY_STATE_PRESSED)
            play(bucketFor(e.keycode, false));
        else if (cfg.release && cfg.release->value())
            play(bucketFor(e.keycode, true));
    }
}

APICALL EXPORT std::string PLUGIN_API_VERSION() {
    return HYPRLAND_API_VERSION;
}

APICALL EXPORT PLUGIN_DESCRIPTION_INFO PLUGIN_INIT(HANDLE handle) {
    PHANDLE = handle;

    if (std::string(__hyprland_api_get_hash()) != __hyprland_api_get_client_hash()) {
        HyprlandAPI::addNotification(PHANDLE, "[keysounds] built for another Hyprland: rebuild it from Ryoku Settings > Windows > Window manager", CHyprColor{1.0, 0.2, 0.2, 1.0}, 6000);
        throw std::runtime_error("[keysounds] version mismatch");
    }

    cfg.enabled = makeShared<Config::Values::CBoolValue>("plugin:keysounds:enabled", "play a sound on every key press", true);
    cfg.profile = makeShared<Config::Values::CStringValue>("plugin:keysounds:profile", "the sample profile: a directory under $XDG_DATA_HOME/ryoku/keysounds or /usr/share/ryoku/keysounds", DEFAULT_PROFILE);
    cfg.volume  = makeShared<Config::Values::CFloatValue>("plugin:keysounds:volume", "playback volume, 0 to 1", 0.6F, Config::Values::SFloatValueOptions{.min = 0.F, .max = 1.F});
    cfg.release = makeShared<Config::Values::CBoolValue>("plugin:keysounds:release", "also play the key-up sample", true);
    HyprlandAPI::addConfigValueV2(PHANDLE, cfg.enabled);
    HyprlandAPI::addConfigValueV2(PHANDLE, cfg.profile);
    HyprlandAPI::addConfigValueV2(PHANDLE, cfg.volume);
    HyprlandAPI::addConfigValueV2(PHANDLE, cfg.release);

    if (ca_context_create(&ctx) == CA_SUCCESS) {
        ca_context_change_props(ctx, CA_PROP_APPLICATION_NAME, "Ryoku Key Sounds", CA_PROP_APPLICATION_ID, "dev.ryoku.keysounds", CA_PROP_APPLICATION_ICON_NAME, "input-keyboard", nullptr);
    } else {
        ctx = nullptr;
        HyprlandAPI::addNotification(PHANDLE, "[keysounds] no sound output (libcanberra could not open a context)", CHyprColor{1.0, 0.6, 0.2, 1.0}, 6000);
    }

    keyListener = Event::bus()->m_events.input.keyboard.key.listen([](IKeyboard::SKeyEvent e, Event::SCallbackInfo&) { onKey(e); });
    loadBank();

    return {"keysounds", "Keyboard sounds on every key press", "Ryoku", "1.0"};
}

APICALL EXPORT void PLUGIN_EXIT() {
    keyListener.reset();
    if (ctx) {
        ca_context_destroy(ctx);
        ctx = nullptr;
    }
}
