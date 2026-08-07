#!/bin/sh
set -eu

password=$(cat /mosquitto/data/admin-password)
mosquitto_ctrl \
    --host 127.0.0.1 \
    --port 1883 \
    --username admin \
    --pw "$password" \
    dynsec getDefaultACLAccess >/dev/null 2>&1
