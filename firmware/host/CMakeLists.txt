cmake_minimum_required(VERSION 3.16)
project(desk_contract_host_tests C)

enable_testing()

add_executable(
    desk_contract_test
    test_contract.c
    ../components/desk_contract/desk_contract.c
    ../components/desk_rotation/desk_rotation.c
)
target_include_directories(
    desk_contract_test
    PRIVATE ../components/desk_contract/include
    PRIVATE ../components/desk_rotation/include
)
target_compile_features(desk_contract_test PRIVATE c_std_11)
target_compile_options(desk_contract_test PRIVATE -Wall -Wextra -Werror -pedantic)

add_test(NAME desk_contract_test COMMAND desk_contract_test)
