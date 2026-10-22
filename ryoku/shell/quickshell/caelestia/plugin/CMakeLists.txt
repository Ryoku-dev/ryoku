cmake_minimum_required(VERSION 3.16)
project(Caelestia)

# CUtils.version reads this; the ported tree has no top level project to set it.
if(NOT DEFINED DISPLAY_VERSION)
    set(DISPLAY_VERSION "ryoku-6769205")
endif()

# Unity builds compile many small translation units together, which speeds up
# cold builds of the plugin. Opt-in: the installer enables it, while CI and
# packaging keep per-TU builds for cache friendliness.
option(CAELESTIA_UNITY_BUILD "Compile plugin sources as CMake unity builds" OFF)
if(CAELESTIA_UNITY_BUILD)
    set(CMAKE_UNITY_BUILD ON)
    set(CMAKE_UNITY_BUILD_BATCH_SIZE 16)
endif()

# Normally inherited from the top level project; defaulted here so the plugin
# can still be configured on its own.
if(NOT DEFINED INSTALL_QMLDIR)
    set(INSTALL_QMLDIR "usr/lib/qt6/qml" CACHE STRING "QML install dir")
endif()
if(NOT DEFINED INSTALL_QSCONFDIR)
    set(INSTALL_QSCONFDIR "etc/xdg/quickshell/caelestia" CACHE STRING "Quickshell config install dir")
endif()

find_package(Qt6 REQUIRED COMPONENTS ShaderTools Core Qml Gui Quick QuickControls2 Concurrent Sql Network DBus)

find_package(PkgConfig REQUIRED)
pkg_check_modules(Qalculate IMPORTED_TARGET libqalculate REQUIRED)
pkg_check_modules(Pipewire IMPORTED_TARGET libpipewire-0.3 REQUIRED)
pkg_check_modules(Aubio IMPORTED_TARGET aubio REQUIRED)
pkg_check_modules(Cava IMPORTED_TARGET libcava QUIET)
if(NOT Cava_FOUND)
    pkg_check_modules(Cava IMPORTED_TARGET cava QUIET)
endif()
if(NOT Cava_FOUND)
    # Arch's cava package doesn't ship a .pc file -- find the library
    # and header directly as a fallback.
    find_library(Cava_LIBRARY NAMES cava libcava)
    find_path(Cava_INCLUDE_DIR NAMES cava/cavacore.h)
    if(Cava_LIBRARY AND Cava_INCLUDE_DIR)
        add_library(PkgConfig::Cava UNKNOWN IMPORTED)
        set_target_properties(PkgConfig::Cava PROPERTIES
            IMPORTED_LOCATION "${Cava_LIBRARY}"
            INTERFACE_INCLUDE_DIRECTORIES "${Cava_INCLUDE_DIR}"
        )
        set(Cava_FOUND TRUE)
        message(STATUS "Found cava (via find_library): ${Cava_LIBRARY}")
    endif()
endif()

# The provider is compiled only when cava's headers and library are there, so a build
# without them ships a shell whose visualiser is dead: no error, nothing in the package
# to say so, and at runtime only "CavaProvider unavailable, visualiser disabled" in the
# journal. A distribution build that means to include it asks for it and fails here
# instead, where the message can name what is missing. A developer build leaves it off
# and gets the provider or not, as the machine allows.
option(REQUIRE_CAVA "Fail the configure when libcava is missing" OFF)
if(NOT Cava_FOUND AND REQUIRE_CAVA)
    message(FATAL_ERROR
        "libcava was not found and REQUIRE_CAVA is on: this build would leave out the "
        "audio visualiser. Install libcava (its headers, its shared library and its "
        "pkg-config file) and configure again. Note that Arch's `cava` package ships only "
        "the cava executable - the library comes from the separate libcava package.")
endif()
include(cmake/sensorslib.cmake)

set(QT_QML_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml")
qt_standard_project_setup(REQUIRES 6.9)

include(cmake/pch.cmake)
include(cmake/qml-module.cmake)

# Header only helpers
add_library(caelestia-util INTERFACE)
target_include_directories(caelestia-util INTERFACE "${CMAKE_CURRENT_SOURCE_DIR}/include")

# M3Shapes (Material 3 shapes, Apache-2.0, vendored at the revision the Caelestia
# tree pins) provides the M3Shapes QML module the shell imports.
add_subdirectory(vendor/m3shapes)

add_subdirectory(src/Caelestia)
