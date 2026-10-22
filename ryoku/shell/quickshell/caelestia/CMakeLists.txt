cmake_minimum_required(VERSION 3.19)

if(NOT DEFINED VERSION)
    set(_VERSION_ENV "${CMAKE_SOURCE_DIR}/../.github/version.env")
    if(EXISTS "${_VERSION_ENV}")
        file(STRINGS "${_VERSION_ENV}" _VER_LINE REGEX "^VERSION=")
        if(_VER_LINE)
            string(REGEX REPLACE "^VERSION=" "" VERSION "${_VER_LINE}")
        endif()
        set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS "${_VERSION_ENV}")
    endif()

    if(NOT VERSION OR "${VERSION}" STREQUAL "" OR "${VERSION}" STREQUAL "rolling")
        execute_process(COMMAND git describe --tags --match "v*" --abbrev=0
            WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
            OUTPUT_VARIABLE VERSION
            OUTPUT_STRIP_TRAILING_WHITESPACE
            ERROR_QUIET
        )
    endif()

    string(STRIP "${VERSION}" VERSION)
endif()

if(NOT DEFINED GIT_REVISION)
    set(_REVISION_FILE "${CMAKE_SOURCE_DIR}/../REVISION")
    if(EXISTS "${_REVISION_FILE}")
        file(STRINGS "${_REVISION_FILE}" GIT_REVISION LIMIT_COUNT 1)
        string(STRIP "${GIT_REVISION}" GIT_REVISION)
        set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS "${_REVISION_FILE}")
    else()
        execute_process(COMMAND git rev-parse HEAD
            WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
            OUTPUT_VARIABLE GIT_REVISION
            OUTPUT_STRIP_TRAILING_WHITESPACE
            ERROR_QUIET
        )
    endif()

    if("${GIT_REVISION}" STREQUAL "")
        set(GIT_REVISION "unknown")
    endif()
endif()

set(DISPLAY_VERSION "${VERSION}")

execute_process(COMMAND git rev-parse --abbrev-ref HEAD
    WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
    OUTPUT_VARIABLE _GIT_BRANCH
    OUTPUT_STRIP_TRAILING_WHITESPACE ERROR_QUIET
)
if("${_GIT_BRANCH}" STREQUAL "dev")
    execute_process(COMMAND git rev-parse --short HEAD
        WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
        OUTPUT_VARIABLE _SHORT_HASH
        OUTPUT_STRIP_TRAILING_WHITESPACE ERROR_QUIET
    )
    if(NOT "${_SHORT_HASH}" STREQUAL "")
        set(DISPLAY_VERSION "${VERSION}-dev+${_SHORT_HASH}")
    endif()
endif()

string(REGEX REPLACE "^v" "" VERSION "${VERSION}")
string(REGEX REPLACE "^v" "" DISPLAY_VERSION "${DISPLAY_VERSION}")

if(VERSION)
    project(caelestia-shell VERSION ${VERSION} LANGUAGES C CXX)
else()
    project(caelestia-shell LANGUAGES C CXX)
endif()

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

find_program(CCACHE_PROGRAM ccache)
if(CCACHE_PROGRAM)
    set(CMAKE_C_COMPILER_LAUNCHER "${CCACHE_PROGRAM}")
    set(CMAKE_CXX_COMPILER_LAUNCHER "${CCACHE_PROGRAM}")
    message(STATUS "ccache found, compiler caching enabled")
endif()

set(CMAKE_RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/lib")

set(DISTRIBUTOR "Unset" CACHE STRING "Distributor")
set(ENABLE_MODULES "extras;plugin;shell;m3shapes" CACHE STRING "Modules to build/install")
set(INSTALL_BINDIR "usr/bin" CACHE STRING "Executable install dir")
set(INSTALL_LIBDIR "usr/lib/caelestia" CACHE STRING "Library install dir")
set(INSTALL_DATADIR "usr/share/caelestia" CACHE STRING "Shared Caelestia data install dir")
set(INSTALL_QMLDIR "usr/lib/qt6/qml" CACHE STRING "QML install dir")
set(INSTALL_QSCONFDIR "etc/xdg/quickshell/caelestia" CACHE STRING "Quickshell config install dir")
option(CAELESTIA_OFFLINE "Disallow downloading build dependencies" OFF)
option(CAELESTIA_PACKAGE "Install package-owned repository assets" OFF)
option(CAELESTIA_ENABLE_SANITIZERS "Enable AddressSanitizer and UndefinedBehaviorSanitizer" OFF)
if(CAELESTIA_ENABLE_SANITIZERS)
    add_compile_options(-fsanitize=address,undefined -fno-omit-frame-pointer)
    add_link_options(-fsanitize=address,undefined)
endif()

set(CAELESTIA_M3SHAPES_SOURCE_DIR "" CACHE PATH
    "Pre-fetched M3Shapes source directory")
set(M3SHAPES_REV "unknown")

if("m3shapes" IN_LIST ENABLE_MODULES)
    include(FetchContent)
    set(_M3SHAPES_REVISION_FILE "${CMAKE_CURRENT_SOURCE_DIR}/../scripts/m3shapes-revision")
    file(STRINGS "${_M3SHAPES_REVISION_FILE}" M3SHAPES_REV LIMIT_COUNT 1)
    string(LENGTH "${M3SHAPES_REV}" _M3SHAPES_REVISION_LENGTH)
    if(NOT _M3SHAPES_REVISION_LENGTH EQUAL 40 OR NOT M3SHAPES_REV MATCHES "^[0-9a-f]+$")
        message(FATAL_ERROR "Invalid M3Shapes revision in ${_M3SHAPES_REVISION_FILE}")
    endif()

    option(CAELESTIA_CACHE_DEPS "Keep FetchContent sources outside the build tree" OFF)

    if(CAELESTIA_M3SHAPES_SOURCE_DIR)
        set(_M3SHAPES_SOURCE_DIR "${CAELESTIA_M3SHAPES_SOURCE_DIR}")
    elseif(CAELESTIA_CACHE_DEPS)
        if(DEFINED ENV{CAELESTIA_DEPS_DIR} AND NOT "$ENV{CAELESTIA_DEPS_DIR}" STREQUAL "")
            set(_CAELESTIA_CACHE "$ENV{CAELESTIA_DEPS_DIR}")
        elseif(DEFINED ENV{XDG_CACHE_HOME} AND NOT "$ENV{XDG_CACHE_HOME}" STREQUAL "")
            set(_CAELESTIA_CACHE "$ENV{XDG_CACHE_HOME}/caelestia-kde")
        else()
            set(_CAELESTIA_CACHE "$ENV{HOME}/.cache/caelestia-kde")
        endif()
        set(_M3SHAPES_SOURCE_DIR "${_CAELESTIA_CACHE}/m3shapes-${M3SHAPES_REV}")
    else()
        set(_M3SHAPES_SOURCE_DIR "${CMAKE_BINARY_DIR}/_deps/m3shapes-${M3SHAPES_REV}")
    endif()

    if(CAELESTIA_OFFLINE AND NOT EXISTS "${_M3SHAPES_SOURCE_DIR}/CMakeLists.txt")
        message(FATAL_ERROR
            "Offline build requested, but M3Shapes was not found at ${_M3SHAPES_SOURCE_DIR}. "
            "Run 'make fetch-dependencies' first or set CAELESTIA_M3SHAPES_SOURCE_DIR.")
    endif()

    FetchContent_Declare(
        m3shapes_external
        GIT_REPOSITORY https://github.com/soramanew/m3shapes.git
        GIT_TAG        ${M3SHAPES_REV}
        SOURCE_DIR     "${_M3SHAPES_SOURCE_DIR}"
    )
    if(EXISTS "${_M3SHAPES_SOURCE_DIR}/CMakeLists.txt")
        set(FETCHCONTENT_SOURCE_DIR_M3SHAPES_EXTERNAL "${_M3SHAPES_SOURCE_DIR}")
    endif()
    message(STATUS "Using M3Shapes source: ${_M3SHAPES_SOURCE_DIR}")
    FetchContent_MakeAvailable(m3shapes_external)

    if(TARGET m3shapesplugin)
        set_target_properties(m3shapesplugin PROPERTIES INSTALL_RPATH "$ORIGIN")
    endif()
endif()

add_compile_definitions(
    QT_NO_CAST_FROM_ASCII
    QT_NO_CAST_TO_ASCII
    QT_NO_CAST_FROM_BYTEARRAY
)

add_compile_options(
    -Wall -Wextra -Wpedantic -Wshadow -Wconversion
    -Wold-style-cast -Wnull-dereference -Wdouble-promotion
    -Wformat=2 -Wfloat-equal -Woverloaded-virtual
    -Wsign-conversion -Wredundant-decls -Wswitch
    -Wunreachable-code
)

if(CMAKE_CXX_COMPILER_ID MATCHES "Clang")
    add_compile_options(-Wunused-lambda-capture)
endif()

if("extras" IN_LIST ENABLE_MODULES)
    add_subdirectory(extras)
endif()

if("plugin" IN_LIST ENABLE_MODULES)
    add_subdirectory(plugin)
endif()

if("shell" IN_LIST ENABLE_MODULES)
    set(CAELESTIA_ROOT_DIR "${CMAKE_CURRENT_SOURCE_DIR}/..")

    function(caelestia_install_directory)
        install(DIRECTORY ${ARGV} PATTERN ".git*" EXCLUDE)
    endfunction()

    file(GLOB CAELESTIA_BIN_FILES CONFIGURE_DEPENDS "${CAELESTIA_ROOT_DIR}/src/bin/*")
    if(NOT CAELESTIA_BIN_FILES)
        message(FATAL_ERROR "No CLI wrappers found in ${CAELESTIA_ROOT_DIR}/src/bin")
    endif()
    install(PROGRAMS ${CAELESTIA_BIN_FILES} DESTINATION "${INSTALL_BINDIR}")

    caelestia_install_directory(
        "${CAELESTIA_ROOT_DIR}/src/matugen"
        "${CAELESTIA_ROOT_DIR}/src/schemes"
        DESTINATION "${INSTALL_DATADIR}"
        PATTERN ".git*" EXCLUDE)

    file(GLOB CAELESTIA_STEP_SCRIPTS CONFIGURE_DEPENDS
        "${CAELESTIA_ROOT_DIR}/scripts/[0-9]*.sh")
    if(NOT CAELESTIA_STEP_SCRIPTS)
        message(FATAL_ERROR "No installer step scripts found in ${CAELESTIA_ROOT_DIR}/scripts")
    endif()
    install(PROGRAMS ${CAELESTIA_STEP_SCRIPTS}
        DESTINATION "${INSTALL_DATADIR}/scripts")
    caelestia_install_directory("${CAELESTIA_ROOT_DIR}/scripts/lib"
        DESTINATION "${INSTALL_DATADIR}/scripts"
        PATTERN ".git*" EXCLUDE)
    caelestia_install_directory(
        "${CAELESTIA_ROOT_DIR}/src/dots"
        "${CAELESTIA_ROOT_DIR}/src/dots-extra"
        DESTINATION "${INSTALL_DATADIR}/src"
        PATTERN ".git*" EXCLUDE)
    install(FILES
        "${CAELESTIA_ROOT_DIR}/shell/assets/wallpaper.webp"
        DESTINATION "${INSTALL_DATADIR}/shell/assets")
    install(FILES
        "${CAELESTIA_ROOT_DIR}/assets/org.quickshell.desktop"
        DESTINATION "${INSTALL_DATADIR}/assets")

    foreach(dir assets components modules services utils)
        caelestia_install_directory(${CMAKE_CURRENT_SOURCE_DIR}/${dir}
                DESTINATION ${INSTALL_QSCONFDIR}
                PATTERN ".git*" EXCLUDE)
    endforeach()

    caelestia_install_directory(${CMAKE_CURRENT_SOURCE_DIR}/scripts/
            DESTINATION ${INSTALL_QSCONFDIR}/scripts
            PATTERN "qml-lint-conventions.py" EXCLUDE
            PATTERN "menu_perf_stats.py" EXCLUDE)

    install(PROGRAMS assets/wrap_term_launch.sh DESTINATION "${INSTALL_QSCONFDIR}/assets")

    file(READ shell.qml SHELL_QML)
    string(REPLACE "settings.watchFiles: true" "settings.watchFiles: false" SHELL_QML "${SHELL_QML}")
    file(WRITE "${CMAKE_BINARY_DIR}/qml/shell.qml" "${SHELL_QML}")
    install(FILES "${CMAKE_BINARY_DIR}/qml/shell.qml" DESTINATION "${INSTALL_QSCONFDIR}")

    install(FILES LICENSE DESTINATION "${INSTALL_QSCONFDIR}")

    if(CAELESTIA_PACKAGE)
        # Anything added here and nowhere else is package-only content. The
        # artifact checks have to allow for it: add the installed prefix to
        # PACKAGE_ONLY_PREFIXES in .github/scripts/packaging_contract.py.
        set(_SDDM_THEME_DIR "usr/share/sddm/themes/caelestia")
        caelestia_install_directory(
            "${CAELESTIA_ROOT_DIR}/src/sddm/themes/full/"
            DESTINATION "${_SDDM_THEME_DIR}"
            )
        install(PROGRAMS
            "${CAELESTIA_ROOT_DIR}/src/sddm/sync.sh"
            DESTINATION "${_SDDM_THEME_DIR}/scripts")
        install(FILES
            "${CAELESTIA_ROOT_DIR}/src/kde/shells/caelestia.desktop/contents/fonts/GoogleSansFlex.ttf"
            DESTINATION "${_SDDM_THEME_DIR}/assets/google-sans-flex")
        caelestia_install_directory(
            "${CAELESTIA_ROOT_DIR}/src/kde/shells/caelestia.desktop"
            DESTINATION "usr/share/plasma/shells"
            )
        caelestia_install_directory(
            "${CAELESTIA_ROOT_DIR}/src/yet-another-monochrome-icon-set"
            DESTINATION "${INSTALL_QSCONFDIR}/assets/icons"
            )
    endif()

    find_package(Qt6 QUIET COMPONENTS LinguistTools)
    if(DEFINED Qt6_VERSION)
        set(CAELESTIA_QT_VERSION "${Qt6_VERSION}")
    else()
        set(CAELESTIA_QT_VERSION "unknown")
    endif()
    set(CAELESTIA_ARTIFACT_ROOT_HASH "pending-install")
    configure_file(
        "${CMAKE_CURRENT_SOURCE_DIR}/build-provenance.json.in"
        "${CMAKE_BINARY_DIR}/build-provenance.json"
        @ONLY)
    install(FILES "${CMAKE_BINARY_DIR}/build-provenance.json"
        DESTINATION "${INSTALL_DATADIR}")

    file(GLOB TS_FILES CONFIGURE_DEPENDS "${CMAKE_CURRENT_SOURCE_DIR}/translations/*.ts")
    file(GLOB PREBUILT_QM_FILES CONFIGURE_DEPENDS "${CMAKE_CURRENT_SOURCE_DIR}/translations/*.qm")

    if(Qt6LinguistTools_FOUND AND TS_FILES)
        set(QM_DIR "${CMAKE_BINARY_DIR}/translations")
        set(QM_FILES "")

        foreach(ts_file ${TS_FILES})
            get_filename_component(ts_name "${ts_file}" NAME_WE)
            set(qm_file "${QM_DIR}/${ts_name}.qm")
            add_custom_command(
                OUTPUT "${qm_file}"
                COMMAND "${CMAKE_COMMAND}" -E make_directory "${QM_DIR}"
                COMMAND Qt6::lrelease -silent "${ts_file}" -qm "${qm_file}"
                DEPENDS "${ts_file}"
                COMMENT "Compiling translation ${ts_name}.qm"
                VERBATIM
            )
            list(APPEND QM_FILES "${qm_file}")
        endforeach()

        add_custom_target(caelestia-translations ALL DEPENDS ${QM_FILES})
        install(FILES ${QM_FILES} DESTINATION "${INSTALL_QSCONFDIR}/translations")
    elseif(PREBUILT_QM_FILES)
        message(STATUS "Qt Linguist tools not found - installing the committed catalogues as-is")
        install(FILES ${PREBUILT_QM_FILES} DESTINATION "${INSTALL_QSCONFDIR}/translations")
    elseif(TS_FILES)
        message(WARNING "Qt Linguist tools not found and no compiled catalogues - the shell will only be available in English")
    endif()
endif()
