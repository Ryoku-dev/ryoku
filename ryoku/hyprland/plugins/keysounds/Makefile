# keysounds: build the plugin against the installed Hyprland headers and, for a
# local build, lay the shipped sample profiles into sounds/. `make keysounds.so`
# is what hyprpm.toml runs on a packaged box, where the profiles are already
# installed under /usr/share/ryoku/keysounds.
#
# The profiles (real switch recordings from the MIT-licensed Mechvibes
# repository) and the importer that cuts them live in the sibling
# ryoku/keysounds tree, so the plugin and the niri key-sounds daemon read one
# copy. MECHVIBES and IMPORTER override the checkout and the importer.

ifeq ($(CXX),g++)
    EXTRA_FLAGS = --no-gnu-unique
else
    EXTRA_FLAGS =
endif

CXXFLAGS ?= -O2
CXXFLAGS += -shared -fPIC -std=c++2b -Wall

# The packs come from the vendored bundle beside the profiles package (see
# release/packages/ryoku-keysounds-profiles/payload.sh for why).
MECHVIBES_BUNDLE = $(CURDIR)/../../../../release/packages/ryoku-keysounds-profiles/mechvibes.bundle
MECHVIBES_COMMIT = 8d18b40792c7ce859a898ce0e840a0c01112888c
MECHVIBES ?= $(or $(XDG_CACHE_HOME),$(HOME)/.cache)/ryoku/mechvibes
PROFILES ?= $(CURDIR)/../../../keysounds/profiles.txt
IMPORTER ?= $(shell command -v ryoku-keysounds-import 2>/dev/null || echo $(CURDIR)/../../../keysounds/ryoku-keysounds-import)

all: keysounds.so sounds

keysounds.so: main.cpp
	$(CXX) $(CXXFLAGS) $(LDFLAGS) $(EXTRA_FLAGS) main.cpp -o keysounds.so `pkg-config --cflags --libs hyprland pixman-1 libdrm libcanberra`

$(MECHVIBES)/src/audio:
	git clone -q "$(MECHVIBES_BUNDLE)" "$(MECHVIBES)"
	git -C "$(MECHVIBES)" checkout -q "$(MECHVIBES_COMMIT)"

sounds: $(PROFILES) $(MECHVIBES)/src/audio
	rm -rf sounds
	grep -vE '^[[:space:]]*(#|$$)' $(PROFILES) | while read -r name pack; do \
	  "$(IMPORTER)" "$(MECHVIBES)/src/audio/$$pack" --name "$$name" --to sounds || exit 1; \
	done
	cp "$(MECHVIBES)/LICENSE" sounds/LICENSE.mechvibes

clean:
	rm -rf keysounds.so sounds

.PHONY: all clean
