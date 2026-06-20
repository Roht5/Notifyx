#!/usr/bin/env bash
set -e

# Target stable Linux Flutter SDK
FLUTTER_VERSION="3.22.2"
FLUTTER_SDK_URL="https://storage.googleapis.com/flutter_infra_release/releases/stable/linux/flutter_linux_${FLUTTER_VERSION}-stable.tar.xz"

echo "=== Render Custom Flutter Web Build Script ==="
echo "Target Flutter version: ${FLUTTER_VERSION}"

# Create development folder in user home
SDK_DIR="$HOME/development"
mkdir -p "$SDK_DIR"

if [ ! -d "$SDK_DIR/flutter" ]; then
  echo "Downloading Flutter SDK..."
  curl -L "$FLUTTER_SDK_URL" -o flutter.tar.xz
  echo "Extracting Flutter SDK to $SDK_DIR..."
  tar -xf flutter.tar.xz -C "$SDK_DIR"
  rm flutter.tar.xz
else
  echo "Using cached Flutter SDK."
fi

# Add Flutter binary to PATH
export PATH="$PATH:$SDK_DIR/flutter/bin"

echo "Checking Flutter version..."
flutter --version

echo "Enabling Web support..."
flutter config --enable-web

echo "Fetching dependencies..."
flutter pub get

echo "Building Flutter Web application (Canvaskit renderer)..."
# Build with CanvaKit renderer for premium dashboard performance and inject production URLs
flutter build web --release --web-renderer canvaskit \
  --dart-define=API_BASE_URL=https://notifyx-api.onrender.com \
  --dart-define=WS_BASE_URL=wss://notifyx-api.onrender.com

echo "=== Build Completed Successfully ==="
