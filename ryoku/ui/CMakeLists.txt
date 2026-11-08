# bytecode only, the C++ variant cost memory and linked Qt private API for no load gain
cmake_minimum_required(VERSION 3.28)

project(ryoku-ui LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_VISIBILITY_PRESET hidden)
set(CMAKE_VISIBILITY_INLINES_HIDDEN ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Qml Quick)

qt_standard_project_setup(REQUIRES 6.5)
# lib/, art/ and shaders/ hold no .qml so this policy changes nothing
if(QT_KNOWN_POLICY_QTP0004)
    qt_policy(SET QTP0004 NEW)
endif()

set(QT_QML_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml")

function(ryoku_ui_module target uri subdir)
    cmake_parse_arguments(PARSE_ARGV 3 arg "" "" "RESOURCES")
    set(dir "${CMAKE_CURRENT_SOURCE_DIR}")
    set(prefix "")
    if(subdir)
        string(APPEND dir "/${subdir}")
        set(prefix "${subdir}/")
    endif()
    set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS "${dir}/qmldir")

    file(STRINGS "${dir}/qmldir" entries)
    set(files "")
    foreach(entry IN LISTS entries)
        string(STRIP "${entry}" entry)
        if(entry STREQUAL "" OR entry MATCHES "^#" OR entry MATCHES "^module ")
            continue()
        endif()
        set(singleton FALSE)
        if(entry MATCHES "^singleton ([A-Za-z_][A-Za-z0-9_]*) ([0-9]+\\.[0-9]+ )?([^ ]+)$")
            set(singleton TRUE)
            set(name "${CMAKE_MATCH_1}")
            set(file "${CMAKE_MATCH_3}")
        elseif(entry MATCHES "^([A-Za-z_][A-Za-z0-9_]*) [0-9]+\\.[0-9]+ ([^ ]+)$")
            set(name "${CMAKE_MATCH_1}")
            set(file "${CMAKE_MATCH_2}")
        else()
            message(FATAL_ERROR "${dir}/qmldir: no rule for '${entry}'; teach ryoku/ui/CMakeLists.txt")
        endif()
        set(path "${prefix}${file}")
        set_source_files_properties("${path}" PROPERTIES
            QT_QML_SOURCE_TYPENAME "${name}"
            QT_QML_SOURCE_VERSIONS "1.0")
        if(singleton)
            set_source_files_properties("${path}" PROPERTIES QT_QML_SINGLETON_TYPE TRUE)
        endif()
        list(APPEND files "${path}")
    endforeach()

    file(GLOB helpers CONFIGURE_DEPENDS RELATIVE "${CMAKE_CURRENT_SOURCE_DIR}"
        "${dir}/*.js" "${dir}/lib/*.js")
    foreach(path IN LISTS helpers)
        if(NOT path IN_LIST files)
            set_source_files_properties("${path}" PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
            list(APPEND files "${path}")
        endif()
    endforeach()

    if(subdir)
        foreach(path IN LISTS files arg_RESOURCES)
            string(REPLACE "${prefix}" "" alias "${path}")
            set_source_files_properties("${path}" PROPERTIES QT_RESOURCE_ALIAS "${alias}")
        endforeach()
    endif()

    string(REPLACE "." "/" uri_path "${uri}")
    qt_add_qml_module(${target}
        URI ${uri}
        VERSION 1.0
        PLUGIN_TARGET ${target}
        OUTPUT_DIRECTORY "${QT_QML_OUTPUT_DIRECTORY}/${uri_path}"
        QML_FILES ${files}
        RESOURCES ${arg_RESOURCES}
    )
    set_target_properties(${target} PROPERTIES QT_QMLCACHEGEN_ARGUMENTS "--only-bytecode")
endfunction()

ryoku_ui_module(ryoku-ui-singletons Ryoku.Ui.Singletons Singletons)

file(GLOB ui_resources CONFIGURE_DEPENDS RELATIVE "${CMAKE_CURRENT_SOURCE_DIR}"
    "${CMAKE_CURRENT_SOURCE_DIR}/*.png"
    "${CMAKE_CURRENT_SOURCE_DIR}/art/*.svg"
    "${CMAKE_CURRENT_SOURCE_DIR}/shaders/*.qsb")
ryoku_ui_module(ryoku-ui Ryoku.Ui "" RESOURCES ${ui_resources})
