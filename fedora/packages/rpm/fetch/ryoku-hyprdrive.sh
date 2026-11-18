#!/usr/bin/env bash
# Hyprdrive lives in its own repository; Ryoku builds its ryoku branch.
export package=ryoku-hyprdrive
export repo=https://github.com/dodo986/hyprdrive-ryoku.git
export source_name=hyprdrive
export branch=ryoku
source "$(dirname "$0")/hypr-plugin"
