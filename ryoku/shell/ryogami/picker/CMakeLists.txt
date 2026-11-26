# QML file names must be unique across qml/: each becomes a type named after its
# file, and one starting with `pragma Singleton` registers as a singleton.
cmake_minimum_required(VERSION 3.28)

project(ryogami-picker LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 6.6 REQUIRED COMPONENTS Quick ShaderTools Multimedia GuiPrivate)

# The header alone: src/core/gpudevices.cpp defines VK_NO_PROTOTYPES and resolves
# every entry point through QLibrary at run time, so the loader is a runtime
# concern. Asking for it here instead of letting the include fail at compile time
# puts the failure in the 0.8 s configure step with the missing piece named.
find_path(VULKAN_INCLUDE_DIR vulkan/vulkan.h REQUIRED)

qt_standard_project_setup(REQUIRES 6.6)

file(GLOB_RECURSE picker_sources CONFIGURE_DEPENDS
    RELATIVE "${CMAKE_CURRENT_SOURCE_DIR}"
    src/*.cpp src/*.h)
set(picker_test_sources
    "src/tests/cardinstancesanitize_test.cpp"
    "src/tests/decklayout_test.cpp"
    "src/tests/hexlayout_test.cpp"
    "src/tests/previewyuv_test.cpp"
    "src/tests/thumbdecoder_test.cpp")
list(REMOVE_ITEM picker_sources ${picker_test_sources})
file(GLOB_RECURSE picker_qml CONFIGURE_DEPENDS
    RELATIVE "${CMAKE_CURRENT_SOURCE_DIR}"
    qml/*.qml qml/*.js)
file(GLOB picker_shaders CONFIGURE_DEPENDS
    RELATIVE "${CMAKE_CURRENT_SOURCE_DIR}"
    shaders/*.vert shaders/*.frag)
file(GLOB picker_fonts CONFIGURE_DEPENDS
    RELATIVE "${CMAKE_CURRENT_SOURCE_DIR}"
    qml/theme/fonts/*.ttf)

# QML files sit at the module root so their implicit import is the module and sees the C++ types.
foreach(font_file IN LISTS picker_fonts)
    get_filename_component(font_name "${font_file}" NAME)
    set_source_files_properties("${font_file}" PROPERTIES QT_RESOURCE_ALIAS "fonts/${font_name}")
endforeach()

# The Ryoku mark the masthead wears, read from the brand folder so the repo keeps one copy.
set(picker_brand "${CMAKE_CURRENT_SOURCE_DIR}/../../../assets/brand/logo-mark-v2.png")
set_source_files_properties("${picker_brand}" PROPERTIES QT_RESOURCE_ALIAS "brand/logo-mark.png")

foreach(qml_file IN LISTS picker_qml)
    get_filename_component(qml_name "${qml_file}" NAME)
    set_source_files_properties("${qml_file}" PROPERTIES QT_RESOURCE_ALIAS "${qml_name}")
    file(STRINGS "${qml_file}" singleton_pragma LIMIT_COUNT 1 REGEX "^pragma Singleton")
    if(singleton_pragma)
        set_source_files_properties("${qml_file}" PROPERTIES QT_QML_SINGLETON_TYPE TRUE)
    endif()
endforeach()

# Qt 6.12's qmlcachegen segfaults recording lookup signatures for its new AOT
# validation code (PlaylistBrowsePopover.qml trips it whenever the system QML
# path is on the import list, as Arch's cmake puts it). Skip that code: the
# module compiles the way it did on 6.11. Older Qt has no such keyword.
set(picker_qml_module_options)
if(Qt6_VERSION VERSION_GREATER_EQUAL 6.12)
    list(APPEND picker_qml_module_options NO_GENERATE_AOT_VALIDATION)
endif()

qt_add_qml_module(ryogami-picker
    URI Ryoku.Ryogami
    VERSION 1.0
    OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Ryogami"
    SOURCES ${picker_sources}
    QML_FILES ${picker_qml}
    RESOURCES ${picker_fonts} ${picker_brand}
    ${picker_qml_module_options}
)

# qmltyperegistrar includes the QML_ELEMENT headers by bare name.
file(GLOB picker_source_dirs LIST_DIRECTORIES true "${CMAKE_CURRENT_SOURCE_DIR}/src/*")
target_include_directories(ryogami-picker PRIVATE ${picker_source_dirs} ${VULKAN_INCLUDE_DIR})

# QRhi sits behind the GuiPrivate include path; only its exported classes are used.
target_link_libraries(ryogami-picker PRIVATE Qt6::Quick Qt6::Multimedia Qt6::GuiPrivate)

option(RYOGAMI_PICKER_TESTS "Build Ryogami picker CPU invariant tests" OFF)
if(RYOGAMI_PICKER_TESTS)
    enable_testing()
    add_executable(ryogami-cardinstancesanitize-test
        src/tests/cardinstancesanitize_test.cpp
        src/render/cardinstancesanitize.cpp)
    target_include_directories(ryogami-cardinstancesanitize-test PRIVATE src/render)
    add_test(NAME ryogami-cardinstancesanitize COMMAND ryogami-cardinstancesanitize-test)
    add_executable(ryogami-previewyuv-test
        src/tests/previewyuv_test.cpp
        src/render/previewyuv.cpp)
    target_link_libraries(ryogami-previewyuv-test PRIVATE Qt6::Gui)
    add_test(NAME ryogami-previewyuv COMMAND ryogami-previewyuv-test)
    add_executable(ryogami-decklayout-test
        src/tests/decklayout_test.cpp
        src/layouts/decklayout.cpp)
    target_include_directories(ryogami-decklayout-test PRIVATE src/layouts src/render src/scene)
    target_link_libraries(ryogami-decklayout-test PRIVATE Qt6::Gui)
    add_test(NAME ryogami-decklayout COMMAND ryogami-decklayout-test)
    add_executable(ryogami-hexlayout-test
        src/tests/hexlayout_test.cpp
        src/layouts/hexlayout.cpp
        src/layouts/geometry.cpp)
    target_include_directories(ryogami-hexlayout-test PRIVATE src/layouts src/render src/scene)
    target_link_libraries(ryogami-hexlayout-test PRIVATE Qt6::Gui)
    add_test(NAME ryogami-hexlayout COMMAND ryogami-hexlayout-test)
    add_executable(ryogami-thumbdecoder-test
        src/tests/thumbdecoder_test.cpp
        src/render/thumbdecoder.cpp
        src/render/thumbdecoder.h)
    target_include_directories(ryogami-thumbdecoder-test PRIVATE src/render)
    target_link_libraries(ryogami-thumbdecoder-test PRIVATE Qt6::Gui)
    add_test(NAME ryogami-thumbdecoder COMMAND ryogami-thumbdecoder-test)
endif()

# Relocatable: the plugin finds its backing library through an $ORIGIN rpath.
set_target_properties(ryogami-picker ryogami-pickerplugin PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Ryogami")
set_target_properties(ryogami-pickerplugin PROPERTIES
    INSTALL_RPATH "$ORIGIN"
    BUILD_WITH_INSTALL_RPATH ON)

qt_add_shaders(ryogami-picker "ryogami_shaders"
    GLSL "330,300es"
    PREFIX "/"
    FILES ${picker_shaders}
)
