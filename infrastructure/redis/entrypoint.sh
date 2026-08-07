#!/bin/sh
set -eu

if [ -n "${REDIS_PASSWORD:-}" ]; then
    exec redis-server --appendonly yes --appendfsync everysec --requirepass "$REDIS_PASSWORD"
fi

exec redis-server --appendonly yes --appendfsync everysec

