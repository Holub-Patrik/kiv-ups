cmake_minimum_required(VERSION 4.0)

project(kiv-ups)

option(ASAN "Use address sanitizer" OFF)
option(MSAN "Use memory sanitizer" OFF)
option(TSAN "Use thread sanitizer" OFF)
option(UBSAN "Use undefined behaviour sanitizer" OFF)

if(CMAKE_CXX_COMPILER_ID STREQUAL "Clang")
    message("-- Clang detected, using libc++")
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -stdlib=libc++")
    set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -stdlib=libc++ -lc++abi")
endif()

function(apply_common_settings TARGET)
    target_compile_features(${TARGET} PRIVATE cxx_std_23)

    target_compile_options(${TARGET} PRIVATE 
        $<$<BOOL:${ASAN}>:-fsanitize=address>
        $<$<BOOL:${MSAN}>:-fsanitize=memory>
        $<$<BOOL:${UBSAN}>:-fsanitize=undefined>
        $<$<BOOL:${TSAN}>:-fsanitize=thread>
        $<$<CONFIG:Debug>:-gdwarf-5>
        $<$<CONFIG:Release>:-march=native>
        $<$<CONFIG:RelWithDebInfo>:-gdwarf-5>
    )

    target_link_libraries(${TARGET} PRIVATE
    )

    target_link_options(${TARGET} PRIVATE
        $<$<BOOL:${ASAN}>:-fsanitize=address>
        $<$<BOOL:${MSAN}>:-fsanitize=memory>
        $<$<BOOL:${UBSAN}>:-fsanitize=undefined>
        $<$<BOOL:${TSAN}>:-fsanitize=thread>
    )   
endfunction()

file(GLOB_RECURSE sources 
	src/*.c 
	src/*.cpp 
	src/*.h 
	src/*.hpp
)

add_executable(poker-server ${sources})
apply_common_settings(poker-server)

# experiments folder
file(GLOB_RECURSE exp_sources
    exp/*.c
	exp/*.cpp 
	exp/*.h 
	exp/*.hpp
)

if(exp_sources)
    add_executable(experiment ${exp_sources})
    apply_common_settings(experiment)
endif()
