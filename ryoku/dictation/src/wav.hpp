#pragma once

#include <string>
#include <vector>

namespace ryoku {

// Reads a WAV at any sample rate, downmixes to mono and linearly resamples it
// to the 16 kHz the engine requires. The library links no resampler, so this is
// where a stray 48 kHz test clip is made acceptable; the live path already
// captures at 16 kHz and never comes through here.
bool readWav16k(const std::string& path, std::vector<float>& out, std::string& err);

} // namespace ryoku
