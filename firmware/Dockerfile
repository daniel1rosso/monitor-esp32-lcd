# syntax=docker/dockerfile:1.7

FROM espressif/idf:v6.0.2 AS build
WORKDIR /project
COPY . .
RUN idf.py set-target esp32c6 && idf.py build

FROM scratch AS artifact
COPY --from=build /project/build/desk_monitor_firmware.bin /
COPY --from=build /project/build/bootloader/bootloader.bin /
COPY --from=build /project/build/partition_table/partition-table.bin /

