#!/bin/sh
set -eu

config=/mosquitto/data/dynamic-security.json
password_file=/mosquitto/data/admin-password
backend_password_file=/mosquitto/data/backend-password

if [ ! -f "$config" ]; then
    password=${MOSQUITTO_ADMIN_PASSWORD:-}
    if [ -z "$password" ]; then
        password=$(head -c 32 /dev/urandom | base64)
    fi
    umask 077
    printf '%s' "$password" > "$password_file"
    mosquitto_ctrl dynsec init "$config" admin "$password"
fi

if [ ! -s "$backend_password_file" ]; then
    umask 077
    head -c 32 /dev/urandom | base64 | tr -d '\n' > "$backend_password_file"
fi

if [ ! -s "$password_file" ] || [ ! -s "$backend_password_file" ]; then
    echo "Mosquitto admin password file is missing" >&2
    exit 1
fi

chown -R 1883:1883 /mosquitto/data
chmod 0440 "$password_file" "$backend_password_file"
