#!/bin/bash

# BingPaperDesktop Build Script for Unix-like systems (macOS/Linux)
# This script builds the application for multiple platforms.

set -e

# Change to the project root directory
cd "$(dirname "$0")/.."

APP_NAME="BingPaperDesktop"
OUTPUT_DIR="build/bin"
WAILS_BIN=$(which wails)

# Check if Wails CLI is installed
if [ -z "$WAILS_BIN" ]; then
    echo "Error: Wails CLI not found. Please install it first:"
    echo "go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
fi

echo "--- Starting build process for $APP_NAME ---"

# Function to clean build directory
clean() {
    echo "Cleaning build directory..."
    rm -rf "$OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR"
}

# Function to build for a specific platform
build_platform() {
    local platform=$1
    local extra_args=$2
    echo "Building for $platform..."
    $WAILS_BIN build -platform "$platform" $extra_args
}

# Default: Clean before build
clean

# 1. Build for Windows (amd64)
echo "Building Windows standalone executable..."
build_platform "windows/amd64"

if command -v makensis >/dev/null 2>&1; then
    echo "NSIS detected. Generating Windows installer..."
    # Use -s to skip frontend build since it was just built
    build_platform "windows/amd64" "-nsis -s"
else
    echo "NSIS not found. Skipping Windows installer generation."
fi

# 2. Build for Linux (amd64)
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Check if we should use webkit2gtk-4.1 (standard on Ubuntu 24.04+)
    if pkg-config --exists webkit2gtk-4.1 && ! pkg-config --exists webkit2gtk-4.0; then
        echo "Detected webkit2gtk-4.1 but not 4.0. Adding -tags webkit2_41..."
        build_platform "linux/amd64" "-tags webkit2_41"
    else
        build_platform "linux/amd64"
    fi
else
    # If not on Linux, we can still cross-compile for Linux but we don't know the target webkit version
    # Default to standard wails build
    build_platform "linux/amd64"
fi

# 3. Build for macOS (Universal) - Only possible on macOS
if [[ "$OSTYPE" == "darwin"* ]]; then
    build_platform "darwin/universal"
    
    # Generate DMG for macOS
    APP_PATH="$OUTPUT_DIR/$APP_NAME.app"
    if [ -d "$APP_PATH" ]; then
        echo "Generating macOS DMG..."
        DMG_TEMP="build/bin/dmg_temp"
        rm -rf "$DMG_TEMP"
        mkdir -p "$DMG_TEMP"
        cp -R "$APP_PATH" "$DMG_TEMP/"
        ln -s /Applications "$DMG_TEMP/Applications"
        
        hdiutil create -volname "$APP_NAME" -srcfolder "$DMG_TEMP" -ov -format UDZO "$OUTPUT_DIR/$APP_NAME.dmg"
        rm -rf "$DMG_TEMP"
        echo "macOS DMG generated: $OUTPUT_DIR/$APP_NAME.dmg"
    fi
else
    echo "Skipping macOS build & DMG generation (must be on macOS)."
fi

echo "--- Build process completed! ---"
echo "Outputs are located in: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"
