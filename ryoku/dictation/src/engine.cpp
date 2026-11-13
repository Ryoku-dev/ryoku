#include "engine.hpp"

#include <transcribe.h>

#include <cstdio>
#include <cstdlib>
#include <filesystem>
#include <string>
#include <vector>

#include <unistd.h>

namespace ryoku {
namespace {

// The library polls this between decode steps. Kept out of the class so no
// library type leaks into the header.
struct AbortHook {
    static bool check(void* user) {
        auto* engine = static_cast<Engine*>(user);
        return engine && engine->stopRequested();
    }
};

// The engine library and its ggml backend modules live in one directory, and
// that directory is where the optional GPU module appears when a user installs
// the backend package. Candidates are tried in order so the same binary works
// from an install, from a build tree and from a package staging root without
// anyone setting an environment variable: the configured override, the layout
// next to the executable, the system directory, then the library's own guess.
std::vector<std::string> backendCandidates() {
    std::vector<std::string> out;
    if (const char* configured = std::getenv("RYOKU_DICTATION_BACKENDS")) {
        if (*configured)
            out.emplace_back(configured);
    }

    char exe[4096];
    const ssize_t length = readlink("/proc/self/exe", exe, sizeof(exe) - 1);
    if (length > 0) {
        exe[length] = '\0';
        std::filesystem::path dir = std::filesystem::path(exe).parent_path();
        out.push_back((dir.parent_path() / "lib" / "ryoku" / "dictation").string());
        out.push_back((dir / "bin").string());
    }

    if (const char* prefix = std::getenv("RYOKU_DICTATION_PREFIX")) {
        if (*prefix)
            out.push_back(std::string(prefix) + "/usr/lib/ryoku/dictation");
    }
    out.emplace_back("/usr/lib/ryoku/dictation");
    return out;
}

void ensureBackends() {
    static bool done = [] {
        // A directory only counts if it actually holds backend modules: probing
        // an empty one makes the library log a warning about a path that was
        // never meant to be a backend directory.
        const auto hasModules = [](const std::string& dir) {
            std::error_code ec;
            if (!std::filesystem::is_directory(dir, ec))
                return false;
            for (const auto& entry : std::filesystem::directory_iterator(dir, ec)) {
                if (entry.path().filename().string().rfind("libggml", 0) == 0)
                    return true;
            }
            return false;
        };

        // The daemon reports what it can do through its socket; the library's
        // own chatter has no place in a desktop session's stderr.
        transcribe_log_set(nullptr, nullptr);
        for (const auto& dir : backendCandidates()) {
            if (!hasModules(dir))
                continue;
            if (transcribe_init_backends(dir.c_str()) == TRANSCRIBE_OK
                && transcribe_device_count() > 0)
                return true;
        }
        transcribe_init_backends_default();
        return true;
    }();
    (void)done;
}

transcribe_backend_request pickBackend(const std::string& device, std::string& err) {
    ensureBackends();
    if (device == "cpu")
        return TRANSCRIBE_BACKEND_CPU;
    if (device != "gpu")
        return TRANSCRIBE_BACKEND_AUTO;

    for (const auto backend : {TRANSCRIBE_BACKEND_VULKAN, TRANSCRIBE_BACKEND_CUDA,
                               TRANSCRIBE_BACKEND_ROCM}) {
        if (transcribe_backend_available(backend))
            return backend;
    }
    err = "GPU backend is not installed";
    return TRANSCRIBE_BACKEND_AUTO;
}

} // namespace

Engine::~Engine() { unload(); }

void Engine::unload() {
    if (!session)
        return;
    transcribe_session_free(session);
    session = nullptr;
    abortRequested = false;
    abortedFlag = false;
}

bool Engine::load(const std::string& path, const std::string& device, int threads,
                  std::string& err) {
    unload();
    ensureBackends();
    if (path.empty()) {
        err = "no speech model installed";
        return false;
    }

    transcribe_init_backends_default();

    auto backend = pickBackend(device, err);
    if (!err.empty())
        return false;

    transcribe_model_load_params loadParams;
    transcribe_model_load_params_init(&loadParams);
    loadParams.backend = backend;

    transcribe_session_params sessionParams;
    transcribe_session_params_init(&sessionParams);
    sessionParams.n_threads = threads;

    const auto status = transcribe_open(path.c_str(), &loadParams, &sessionParams, &session);
    if (status != TRANSCRIBE_OK) {
        session = nullptr;
        err = std::string("cannot load model: ") + transcribe_status_string(status);
        return false;
    }
    transcribe_set_abort_callback(session, &AbortHook::check, this);

    // Build the compute graphs once, here, against a half second of silence.
    // A GPU backend compiles its shaders on the first real run, which cost 11 s
    // of "Preparing" on a cold box; paying it while the card already says
    // Preparing is the honest place to put it.
    warmUp();
    return true;
}

void Engine::warmUp() {
    static constexpr int samples = 8000; // 0.5 s at 16 kHz
    std::vector<float> silence(samples, 0.0f);
    transcribe_run_params params;
    transcribe_run_params_init(&params);
    params.timestamps = TRANSCRIBE_TIMESTAMPS_NONE;
    // A warm-up that fails is not a load failure: the next real run reports the
    // engine's own error, which is the one the user needs to see.
    (void)transcribe_run(session, silence.data(), samples, &params);
}

std::string Engine::backend() const {
    if (!session)
        return "";
    const char* name = transcribe_model_backend(transcribe_get_model(session));
    return name ? name : "";
}

bool Engine::run(const std::vector<float>& pcm, const std::string& language, std::string& text,
                 std::string& err) {
    text.clear();
    if (!session) {
        err = "engine is not loaded";
        return false;
    }
    if (pcm.empty()) {
        err = "nothing was recorded";
        return false;
    }

    abortRequested = false;
    abortedFlag = false;

    transcribe_run_params params;
    transcribe_run_params_init(&params);
    params.task = TRANSCRIBE_TASK_TRANSCRIBE;
    params.timestamps = TRANSCRIBE_TIMESTAMPS_NONE;
    if (!language.empty())
        params.language = language.c_str();

    const auto status =
        transcribe_run(session, pcm.data(), static_cast<int>(pcm.size()), &params);
    abortedFlag = abortRequested.load();
    if (status != TRANSCRIBE_OK) {
        err = std::string(abortedFlag ? "cancelled: " : "transcription failed: ")
            + transcribe_status_string(status);
        return false;
    }
    const char* full = transcribe_full_text(session);
    text = full ? full : "";
    return true;
}

void Engine::abort() { abortRequested = true; }

bool gpuAvailable(std::string& which) {
    ensureBackends();
    for (const auto backend : {TRANSCRIBE_BACKEND_VULKAN, TRANSCRIBE_BACKEND_CUDA,
                               TRANSCRIBE_BACKEND_ROCM}) {
        if (!transcribe_backend_available(backend))
            continue;
        which = backend == TRANSCRIBE_BACKEND_VULKAN ? "vulkan"
            : backend == TRANSCRIBE_BACKEND_CUDA   ? "cuda"
                                                   : "rocm";
        return true;
    }
    which.clear();
    return false;
}

std::string availableBackends() {
    std::string list = "cpu";
    std::string which;
    if (gpuAvailable(which))
        list += "," + which;
    return list;
}

} // namespace ryoku
