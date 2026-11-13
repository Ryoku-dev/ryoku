#include "capture.hpp"

#include <pipewire/pipewire.h>
#include <spa/param/audio/format-utils.h>
#include <spa/param/audio/raw.h>
#include <spa/pod/builder.h>

#include <cmath>
#include <cstdlib>
#include <cstring>

namespace ryoku {
namespace {

unsigned long long debugFrames = 0;

void onProcess(void* data) { static_cast<Capture*>(data)->pump(); }

void onStateChanged(void* data, pw_stream_state old, pw_stream_state state, const char* error) {
    (void)data;
    if (std::getenv("RYOKU_DICTATE_DEBUG"))
        std::fprintf(stderr, "ryoku-dictate: capture %s -> %s%s%s\n",
                     pw_stream_state_as_string(old), pw_stream_state_as_string(state),
                     error ? ": " : "", error ? error : "");
    else if (state == PW_STREAM_STATE_ERROR && error)
        std::fprintf(stderr, "ryoku-dictate: capture: %s\n", error);
}

const pw_stream_events streamEvents = [] {
    pw_stream_events events{};
    events.version = PW_VERSION_STREAM_EVENTS;
    events.process = onProcess;
    events.state_changed = onStateChanged;
    return events;
}();

} // namespace

Capture::~Capture() { stop(); }

bool Capture::start(const std::string& source, std::string& err) {
    stop();
    pw_init(nullptr, nullptr);

    loop = pw_thread_loop_new("ryoku-dictate", nullptr);
    if (!loop) {
        err = "cannot start the PipeWire loop";
        return false;
    }

    pw_thread_loop_lock(loop);
    context = pw_context_new(pw_thread_loop_get_loop(loop), nullptr, 0);
    if (!context) {
        pw_thread_loop_unlock(loop);
        err = "cannot create a PipeWire context";
        stop();
        return false;
    }
    core = pw_context_connect(context, nullptr, 0);
    if (!core) {
        pw_thread_loop_unlock(loop);
        err = "cannot connect to PipeWire";
        stop();
        return false;
    }

    pw_properties* props = pw_properties_new(
        PW_KEY_MEDIA_TYPE, "Audio",
        PW_KEY_MEDIA_CATEGORY, "Capture",
        PW_KEY_MEDIA_ROLE, "Communication",
        PW_KEY_NODE_NAME, "ryoku-dictate",
        PW_KEY_NODE_DESCRIPTION, "Ryoku voice typing",
        PW_KEY_APP_NAME, "Ryoku",
        nullptr);
    if (!source.empty())
        pw_properties_set(props, PW_KEY_TARGET_OBJECT, source.c_str());

    stream = pw_stream_new(core, "ryoku-dictate", props);
    if (!stream) {
        pw_thread_loop_unlock(loop);
        err = "cannot create the capture stream";
        stop();
        return false;
    }
    listener = new spa_hook{};
    pw_stream_add_listener(stream, listener, &streamEvents, this);

    spa_audio_info_raw info{};
    info.format = SPA_AUDIO_FORMAT_F32;
    info.rate = 16000;
    info.channels = 1;

    uint8_t buffer[1024];
    spa_pod_builder builder = SPA_POD_BUILDER_INIT(buffer, sizeof(buffer));
    const spa_pod* params[1];
    params[0] = spa_format_audio_raw_build(&builder, SPA_PARAM_EnumFormat, &info);

    const auto flags = static_cast<pw_stream_flags>(PW_STREAM_FLAG_AUTOCONNECT
                                                    | PW_STREAM_FLAG_MAP_BUFFERS);
    if (pw_stream_connect(stream, PW_DIRECTION_INPUT, PW_ID_ANY, flags, params, 1) < 0) {
        pw_thread_loop_unlock(loop);
        err = "PipeWire refused the capture stream";
        stop();
        return false;
    }
    pw_thread_loop_unlock(loop);

    if (pw_thread_loop_start(loop) < 0) {
        err = "cannot run the PipeWire loop";
        stop();
        return false;
    }

    {
        std::lock_guard guard(mutex);
        frame.clear();
        frame.reserve(16000 * 180);
    }
    lastRms = 0;
    lastPeak = 0;
    active = true;
    return true;
}

void Capture::pump() {
    pw_buffer* buffer = pw_stream_dequeue_buffer(stream);
    if (!buffer)
        return;
    spa_buffer* spa = buffer->buffer;
    if (std::getenv("RYOKU_DICTATE_DEBUG") && (++debugFrames % 50) == 0)
        std::fprintf(stderr, "ryoku-dictate: frames=%llu size=%u\n",
                     static_cast<unsigned long long>(debugFrames),
                     spa->datas[0].chunk ? spa->datas[0].chunk->size : 0u);
    if (spa->datas[0].data && spa->datas[0].chunk) {
        const auto* bytes = static_cast<const char*>(spa->datas[0].data)
            + spa->datas[0].chunk->offset;
        const size_t count = spa->datas[0].chunk->size / sizeof(float);
        const auto* samples = reinterpret_cast<const float*>(bytes);

        float rms = 0;
        float peak = 0;
        double sum = 0;
        for (size_t i = 0; i < count; i++) {
            const float value = samples[i];
            sum += double(value) * double(value);
            peak = std::max(peak, std::fabs(value));
        }
        if (count > 0)
            rms = float(std::sqrt(sum / double(count)));
        lastRms = rms;
        lastPeak = peak;

        std::lock_guard guard(mutex);
        frame.insert(frame.end(), samples, samples + count);
    }
    pw_stream_queue_buffer(stream, buffer);
}

std::vector<float> Capture::take() {
    std::vector<float> out;
    std::lock_guard guard(mutex);
    out.swap(frame);
    frame.reserve(16000 * 180);
    return out;
}

void Capture::levels(float& rms, float& peak) const {
    rms = lastRms;
    peak = lastPeak;
}

void Capture::stop() {
    active = false;
    if (stream) {
        // Both calls belong to the loop's thread context; the loop is still
        // running, so take its lock around them.
        if (loop)
            pw_thread_loop_lock(loop);
        pw_stream_disconnect(stream);
        pw_stream_destroy(stream);
        if (loop)
            pw_thread_loop_unlock(loop);
        stream = nullptr;
    }
    delete listener;
    listener = nullptr;
    if (loop)
        pw_thread_loop_stop(loop);
    if (core) {
        pw_core_disconnect(core);
        core = nullptr;
    }
    if (context) {
        pw_context_destroy(context);
        context = nullptr;
    }
    if (loop) {
        pw_thread_loop_destroy(loop);
        loop = nullptr;
    }
    // The recorded audio is left in place: the caller takes it after stopping,
    // which is the only ordering that cannot lose the last few hundred ms.
    lastRms = 0;
    lastPeak = 0;
    pw_deinit();
}

} // namespace ryoku
