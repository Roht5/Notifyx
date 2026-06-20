#!/bin/sh
# Exit immediately if migrations fail
set -e

echo "=== Running Migrations ==="
./migrate

echo "=== Starting Notifyx Server ==="
exec ./notifyx
