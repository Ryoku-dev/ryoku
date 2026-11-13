# ryoku-dictate: Ryoku's own voice typing engine.
#
# Capture is PipeWire, insertion is the Wayland virtual-keyboard protocol, and
# the speech engine is transcribe.cpp (MIT, ggml) at a pinned revision. The
# engine is fetched at build time, never vendored into the tree: the package
# build has the network, and an upgrade is one version and one hash.
#
# The binary ships prebuilt in ryoku-dictation, so the installed machine needs
# no toolchain, no Python, and no second process for the meter.
cmake_minimum_required(VERSION 3.20)
project(ryoku-dictate LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)
# The payload flattens the engine and ggml modules into one private directory.
# Each shared object needs its own origin path: the executable's RUNPATH is
# not inherited by transitive dependencies.
set(CMAKE_BUILD_RPATH "$ORIGIN")
set(CMAKE_BUILD_RPATH_USE_ORIGIN ON)
if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release)
endif()

option(RYOKU_DICTATION_GPU "Build the Vulkan ggml backend" OFF)
option(RYOKU_DICTATION_TEST "Build the unit tests" ON)

find_package(PkgConfig REQUIRED)
pkg_check_modules(PIPEWIRE REQUIRED IMPORTED_TARGET libpipewire-0.3)
pkg_check_modules(WAYLAND REQUIRED IMPORTED_TARGET wayland-client)
pkg_check_modules(XKB REQUIRED IMPORTED_TARGET xkbcommon)
find_package(Threads REQUIRED)

# ── the speech engine, pinned ────────────────────────────────────────────────
set(TRANSCRIBE_VERSION 0.4.2)
set(TRANSCRIBE_SHA256 5455374e18adaa0d79ff48104b13109d15dd6002edda999eda1179a86ea09c5b)

set(TRANSCRIBE_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(TRANSCRIBE_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(TRANSCRIBE_BUILD_TOOLS OFF CACHE BOOL "" FORCE)
set(TRANSCRIBE_USE_SYSTEM_BLAS OFF CACHE BOOL "" FORCE)
set(TRANSCRIBE_VULKAN ${RYOKU_DICTATION_GPU} CACHE BOOL "" FORCE)
# The engine and its ggml backends are always shared modules in one directory,
# and the backends are always loadable: that is what makes the CPU/GPU switch a
# setting rather than a second package. The base package ships the CPU module;
# the optional GPU package drops libggml-vulkan.so into the same directory and
# the daemon finds it at startup.
set(TRANSCRIBE_BUILD_SHARED ON CACHE BOOL "" FORCE)
set(TRANSCRIBE_GGML_BACKEND_DL ON CACHE BOOL "" FORCE)
# Upstream's install rules would put ggml's headers, CMake configs and its own
# /usr/lib/libggml.so into a desktop runtime package -- and on a box that already
# carries a ggml build, that collides with it. ggml's install rules are not
# switchable, so the payload copies exactly the files we ship and the rpath is
# written at link time instead.
set(TRANSCRIBE_INSTALL OFF CACHE BOOL "" FORCE)

if(TRANSCRIBE_SRC)
    set(TRANSCRIBE_ROOT "${TRANSCRIBE_SRC}")
    add_subdirectory("${TRANSCRIBE_SRC}" "${CMAKE_BINARY_DIR}/transcribe" EXCLUDE_FROM_ALL)
else()
    include(FetchContent)
    FetchContent_Declare(transcribe
        URL "https://github.com/handy-computer/transcribe.cpp/archive/refs/tags/v${TRANSCRIBE_VERSION}.tar.gz"
        URL_HASH "SHA256=${TRANSCRIBE_SHA256}")
    FetchContent_MakeAvailable(transcribe)
    set(TRANSCRIBE_ROOT "${transcribe_SOURCE_DIR}")
endif()
# Upstream publishes its public headers as ${CMAKE_SOURCE_DIR}/include, which is
# this project's root when it is embedded. Name the real directory.
target_include_directories(transcribe PUBLIC "${TRANSCRIBE_ROOT}/include")

# ── the Wayland protocol we implement ───────────────────────────────────────
find_program(WAYLAND_SCANNER wayland-scanner REQUIRED)
set(VK_XML "${CMAKE_CURRENT_SOURCE_DIR}/proto/virtual-keyboard-unstable-v1.xml")
set(VK_HEADER "${CMAKE_CURRENT_BINARY_DIR}/virtual-keyboard-unstable-v1-client-protocol.h")
set(VK_SOURCE "${CMAKE_CURRENT_BINARY_DIR}/virtual-keyboard-unstable-v1-protocol.c")
add_custom_command(OUTPUT "${VK_HEADER}"
    COMMAND "${WAYLAND_SCANNER}" client-header "${VK_XML}" "${VK_HEADER}"
    DEPENDS "${VK_XML}" VERBATIM)
add_custom_command(OUTPUT "${VK_SOURCE}"
    COMMAND "${WAYLAND_SCANNER}" private-code "${VK_XML}" "${VK_SOURCE}"
    DEPENDS "${VK_XML}" VERBATIM)

add_executable(ryoku-dictate
    src/main.cpp
    src/config.cpp
    src/engine.cpp
    src/capture.cpp
    src/insert.cpp
    src/wav.cpp
    "${VK_SOURCE}"
    "${VK_HEADER}")
target_include_directories(ryoku-dictate PRIVATE
    src
    "${CMAKE_CURRENT_BINARY_DIR}")
target_link_libraries(ryoku-dictate PRIVATE
    transcribe
    PkgConfig::PIPEWIRE
    PkgConfig::WAYLAND
    PkgConfig::XKB
    Threads::Threads)


# The daemon resolves libtranscribe (and the ggml backend modules, which is where
# the optional GPU package drops its file) from one directory beside the install.
# The path is relative to the binary, so the same file works from /usr and from a
# package staging root, and CMake still adds the build-tree paths for dev runs.
target_link_options(ryoku-dictate PRIVATE
    "-Wl,-rpath,$ORIGIN/../lib/ryoku/dictation")

if(RYOKU_DICTATION_TEST)
    enable_testing()
    add_executable(dictate-test
        tests/main.cpp
        src/config.cpp
        src/wav.cpp
        src/insert.cpp
        "${VK_SOURCE}"
        "${VK_HEADER}")
    target_include_directories(dictate-test PRIVATE src "${CMAKE_CURRENT_BINARY_DIR}")
    # insert.cpp needs the protocol, config and wav are pure; no engine, no pw.
    target_link_libraries(dictate-test PRIVATE
        PkgConfig::WAYLAND PkgConfig::XKB Threads::Threads)
    add_test(NAME dictate-unit COMMAND dictate-test)
endif()
