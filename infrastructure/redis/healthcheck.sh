#!/bin/sh
set -eu

if [ -n "${REDIS_PASSWORD:-}" ]; then
    exec redis-cli --no-auth-warning --pass "$REDIS_PASSWORD" ping
fi

exec redis-cli ping

