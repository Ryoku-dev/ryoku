#!/usr/bin/env bash
# Hyprdrive lives in its own repository; Ryoku builds its ryoku branch.
export package=ryoku-hyprdrive
export repo=https://github.com/Ryoku-dev/ryoku-hyprdrive.git
export source_name=hyprdrive
export branch=ryoku
source "$(dirname "$0")/hypr-plugin"
