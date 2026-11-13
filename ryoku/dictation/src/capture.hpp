#pragma once

#include <atomic>
#include <mutex>
#include <string>
#include <vector>

struct pw_thread_loop;
struct pw_stream;
struct pw_context;
struct pw_core;
struct spa_hook;

namespace ryoku {

// One PipeWire capture stream at 16 kHz mono float.
//
// The same buffer feeds the engine and the meter, so the wave on screen is
// exactly what is being transcribed: no second stream on the microphone, no
// cava process, no bridge binary.
class Capture {
public:
    Capture() = default;
    ~Capture();
    Capture(const Capture&) = delete;
    Capture& operator=(const Capture&) = delete;

    // source: a node name to record from, empty for the default source.
    bool start(const std::string& source, std::string& err);
    // Moves everything recorded so far out of the buffer.
    std::vector<float> take();
    void stop();
    bool running() const { return active.load(); }

    // Latest meter reading, 0..1. Written on PipeWire's thread, read by the
    // daemon's loop, so the socket is never written from the RT callback.
    void levels(float& rms, float& peak) const;

    // Internal: PipeWire's process callback lands here.
    void pump();

private:
    void reset();

    pw_thread_loop* loop = nullptr;
    pw_stream* stream = nullptr;
    pw_context* context = nullptr;
    pw_core* core = nullptr;
    spa_hook* listener = nullptr;

    std::mutex mutex;
    std::vector<float> frame;
    std::atomic<bool> active{false};
    std::atomic<float> lastRms{0.0f};
    std::atomic<float> lastPeak{0.0f};
};

} // namespace ryoku
