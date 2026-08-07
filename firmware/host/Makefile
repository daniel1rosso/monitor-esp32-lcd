CC ?= cc
BUILD_DIR ?= build
TARGET := $(BUILD_DIR)/desk_contract_test
SOURCES := test_contract.c ../components/desk_contract/desk_contract.c ../components/desk_rotation/desk_rotation.c
INCLUDES := -I../components/desk_contract/include -I../components/desk_rotation/include
CFLAGS := -std=c11 -Wall -Wextra -Werror -pedantic

.PHONY: all test clean

all: $(TARGET)

$(TARGET): $(SOURCES)
	mkdir -p $(BUILD_DIR)
	$(CC) $(CFLAGS) $(INCLUDES) $(SOURCES) -o $(TARGET)

test: $(TARGET)
	$(TARGET)

clean:
	rm -f $(TARGET)
