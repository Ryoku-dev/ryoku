#pragma once

#include "cardinstance.h"

#include <cstddef>
#include <vector>

std::size_t sanitizeCardInstances(std::vector<CardInstance> &instances, int nearLayers,
                                  int farLayers, bool previewAvailable,
                                  float coordinateLimit = 65536.0f);
