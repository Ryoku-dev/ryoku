#pragma once

#include <atomic>
#include <string>
#include <vector>

struct transcribe_session;

namespace ryoku {

// The speech engine, behind one thin door.
//
// transcribe.cpp is pinned and fetched at build time; nothing above this header
// knows its API. Swapping engines later means rewriting this file, not the
// daemon, the UI or the Hub.
class Engine {
public:
    Engine() = default;
    ~Engine();
    Engine(const Engine&) = delete;
    Engine& operator=(const Engine&) = delete;

    // device: auto | cpu | gpu. "gpu" fails rather than silently running on the
    // CPU, because the user asked for speed and deserves to be told.
    bool load(const std::string& path, const std::string& device, int threads, std::string& err);
    void unload();
    bool ready() const { return session != nullptr; }

    // "vulkan", "cpu", ... as the library reports the active backend.
    std::string backend() const;

    bool run(const std::vector<float>& pcm, const std::string& language, std::string& text,
             std::string& err);
    void abort();
    bool aborted() const { return abortedFlag; }
    bool stopRequested() const { return abortRequested.load(); }

private:
    void warmUp();

    transcribe_session* session = nullptr;
    std::atomic<bool> abortRequested{false};
    std::atomic<bool> abortedFlag{false};
};

// True when at least one non-CPU backend is compiled into this build and found
// a device at runtime. The Hub uses it to offer the toggle at all.
bool gpuAvailable(std::string& which);
std::string availableBackends();

} // namespace ryoku
