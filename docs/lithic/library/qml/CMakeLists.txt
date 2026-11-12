# bytecode only, like Ryoku.Ui: the C++ variant cost memory for no load gain
cmake_minimum_required(VERSION 3.28)

project(lithic-qml LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_VISIBILITY_PRESET hidden)
set(CMAKE_VISIBILITY_INLINES_HIDDEN ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Qml Quick)

qt_standard_project_setup(REQUIRES 6.5)

set(QT_QML_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml")

set(module_dir "${CMAKE_CURRENT_SOURCE_DIR}/Lithic")
set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS "${module_dir}/qmldir")

file(STRINGS "${module_dir}/qmldir" entries)
set(files "")
foreach(entry IN LISTS entries)
    string(STRIP "${entry}" entry)
    if(entry STREQUAL "" OR entry MATCHES "^#" OR entry MATCHES "^module ")
        continue()
    endif()
    if(entry MATCHES "^singleton ([A-Za-z_][A-Za-z0-9_]*) [0-9]+\\.[0-9]+ ([^ ]+)$")
        set(kind singleton)
    elseif(entry MATCHES "^internal ([A-Za-z_][A-Za-z0-9_]*) ([^ ]+)$")
        set(kind internal)
    elseif(entry MATCHES "^([A-Za-z_][A-Za-z0-9_]*) [0-9]+\\.[0-9]+ ([^ ]+)$")
        set(kind type)
    else()
        message(FATAL_ERROR "Lithic/qmldir: no rule for '${entry}'; teach docs/lithic/library/qml/CMakeLists.txt")
    endif()
    set(name "${CMAKE_MATCH_1}")
    set(file "${CMAKE_MATCH_2}")
    set(path "Lithic/${file}")
    set_source_files_properties("${path}" PROPERTIES
        QT_RESOURCE_ALIAS "${file}"
        QT_QML_SOURCE_TYPENAME "${name}")
    if(kind STREQUAL "internal")
        set_source_files_properties("${path}" PROPERTIES QT_QML_INTERNAL_TYPE TRUE)
    else()
        set_source_files_properties("${path}" PROPERTIES QT_QML_SOURCE_VERSIONS "1.0")
    endif()
    if(kind STREQUAL "singleton")
        set_source_files_properties("${path}" PROPERTIES QT_QML_SINGLETON_TYPE TRUE)
    endif()
    list(APPEND files "${path}")
endforeach()

qt_add_qml_module(lithic
    URI Lithic
    VERSION 1.0
    PLUGIN_TARGET lithic
    OUTPUT_DIRECTORY "${QT_QML_OUTPUT_DIRECTORY}/Lithic"
    QML_FILES ${files}
)
set_target_properties(lithic PROPERTIES QT_QMLCACHEGEN_ARGUMENTS "--only-bytecode")

# Lithic.qml loads its faces from ../../go/fonts, which from qrc:/qt/qml/Lithic
# lands on this prefix; the one copy stays in the Go library
file(GLOB fonts CONFIGURE_DEPENDS "${CMAKE_CURRENT_SOURCE_DIR}/../go/fonts/*.ttf")
qt_add_resources(lithic lithic_fonts
    PREFIX "/qt/go/fonts"
    BASE "${CMAKE_CURRENT_SOURCE_DIR}/../go/fonts"
    FILES ${fonts}
)
