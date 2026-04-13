#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" = "" ] || [ "${2:-}" = "" ] || [ "${3:-}" = "" ]; then
  echo "Usage: $0 <version> <binary-path> <release-dir>" >&2
  exit 1
fi

VERSION_RAW="$1"
BINARY_PATH_INPUT="$2"
RELEASE_DIR_INPUT="$3"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
VERSION="${VERSION_RAW#v}"
BINARY_PATH="$BINARY_PATH_INPUT"
if [[ "$BINARY_PATH" != /* ]]; then
  BINARY_PATH="$ROOT_DIR/$BINARY_PATH"
fi
RELEASE_DIR="$RELEASE_DIR_INPUT"
if [[ "$RELEASE_DIR" != /* ]]; then
  RELEASE_DIR="$ROOT_DIR/$RELEASE_DIR"
fi

APP_SLUG="batch-api-check"
APP_NAME="Batch API Check"
PACKAGE_PREFIX="batch-api-check-linux-amd64"
DEB_PACKAGE_NAME="batch-api-check"
DEB_ARCH="amd64"
APPIMAGE_ARCH="x86_64"
ICON_PATH="$ROOT_DIR/assets/appicon.png"
DESKTOP_FILE="$ROOT_DIR/build/linux/batch-api-check.desktop"
APPDATA_FILE="$ROOT_DIR/build/linux/batch-api-check.appdata.xml"
TMP_DIR="$ROOT_DIR/.tmp-linux-package"
TOOLS_DIR="$TMP_DIR/tools"
WORK_DIR="$TMP_DIR/work"
TARBALL_STAGE_DIR="$WORK_DIR/tarball"
DEB_ROOT="$WORK_DIR/deb-root"
APPDIR="$WORK_DIR/AppDir"
APPIMAGE_TOOL="$TOOLS_DIR/linuxdeploy-x86_64.AppImage"
APPIMAGE_PLUGIN="$TOOLS_DIR/linuxdeploy-plugin-appimage-x86_64.AppImage"
APPIMAGE_PLUGIN_LINK="$TOOLS_DIR/linuxdeploy-plugin-appimage"

require_file() {
  local path="$1"
  if [ ! -f "$path" ]; then
    echo "Required file not found: $path" >&2
    exit 1
  fi
}

download_tool() {
  local url="$1"
  local dest="$2"
  if [ -f "$dest" ]; then
    return
  fi
  curl -fsSL "$url" -o "$dest"
  chmod +x "$dest"
}

prepare_workspace() {
  require_file "$BINARY_PATH"
  require_file "$ICON_PATH"
  require_file "$DESKTOP_FILE"
  require_file "$APPDATA_FILE"

  rm -rf "$TMP_DIR"
  mkdir -p "$TOOLS_DIR" "$WORK_DIR" "$RELEASE_DIR"
  chmod +x "$BINARY_PATH"
}

build_tarball() {
  local stage_dir="$TARBALL_STAGE_DIR/$APP_SLUG"
  mkdir -p "$stage_dir"
  install -m 755 "$BINARY_PATH" "$stage_dir/$APP_SLUG"
  install -m 644 "$DESKTOP_FILE" "$stage_dir/$APP_SLUG.desktop"
  install -m 644 "$ICON_PATH" "$stage_dir/$APP_SLUG.png"
  tar -C "$TARBALL_STAGE_DIR" -czf "$RELEASE_DIR/$PACKAGE_PREFIX.tar.gz" "$APP_SLUG"
}

build_deb() {
  local deb_version
  deb_version="$(printf '%s' "$VERSION" | sed 's/-/~/g')"

  mkdir -p \
    "$DEB_ROOT/DEBIAN" \
    "$DEB_ROOT/opt/$APP_SLUG" \
    "$DEB_ROOT/usr/bin" \
    "$DEB_ROOT/usr/share/applications" \
    "$DEB_ROOT/usr/share/icons/hicolor/512x512/apps" \
    "$DEB_ROOT/usr/share/metainfo"

  install -m 755 "$BINARY_PATH" "$DEB_ROOT/opt/$APP_SLUG/$APP_SLUG"
  ln -s "/opt/$APP_SLUG/$APP_SLUG" "$DEB_ROOT/usr/bin/$APP_SLUG"
  install -m 644 "$DESKTOP_FILE" "$DEB_ROOT/usr/share/applications/$APP_SLUG.desktop"
  install -m 644 "$ICON_PATH" "$DEB_ROOT/usr/share/icons/hicolor/512x512/apps/$APP_SLUG.png"
  install -m 644 "$APPDATA_FILE" "$DEB_ROOT/usr/share/metainfo/$APP_SLUG.appdata.xml"

  cat > "$DEB_ROOT/DEBIAN/control" <<EOF
Package: $DEB_PACKAGE_NAME
Version: $deb_version
Section: utils
Priority: optional
Architecture: $DEB_ARCH
Maintainer: ding
Depends: libgtk-3-0, libwebkit2gtk-4.1-0
Homepage: https://github.com/jlwebs/AllApiDeck
Description: Batch API Check desktop client
 Desktop client for importing browser extension accounts,
 extracting tokens, and checking model availability.
EOF

  cat > "$DEB_ROOT/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q /usr/share/icons/hicolor || true
fi
EOF

  cat > "$DEB_ROOT/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q /usr/share/icons/hicolor || true
fi
EOF

  chmod 755 "$DEB_ROOT/DEBIAN/postinst" "$DEB_ROOT/DEBIAN/postrm"
  dpkg-deb --build --root-owner-group "$DEB_ROOT" "$RELEASE_DIR/$PACKAGE_PREFIX.deb"
}

prepare_appdir() {
  mkdir -p \
    "$APPDIR/usr/bin" \
    "$APPDIR/usr/share/applications" \
    "$APPDIR/usr/share/icons/hicolor/512x512/apps" \
    "$APPDIR/usr/share/metainfo"

  install -m 755 "$BINARY_PATH" "$APPDIR/usr/bin/$APP_SLUG"
  install -m 644 "$DESKTOP_FILE" "$APPDIR/usr/share/applications/$APP_SLUG.desktop"
  install -m 644 "$ICON_PATH" "$APPDIR/usr/share/icons/hicolor/512x512/apps/$APP_SLUG.png"
  install -m 644 "$APPDATA_FILE" "$APPDIR/usr/share/metainfo/$APP_SLUG.appdata.xml"
  install -m 644 "$DESKTOP_FILE" "$APPDIR/$APP_SLUG.desktop"
  install -m 644 "$ICON_PATH" "$APPDIR/$APP_SLUG.png"

  cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/batch-api-check" "$@"
EOF
  chmod 755 "$APPDIR/AppRun"
}

build_appimage() {
  download_tool \
    "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage" \
    "$APPIMAGE_TOOL"
  download_tool \
    "https://github.com/linuxdeploy/linuxdeploy-plugin-appimage/releases/download/continuous/linuxdeploy-plugin-appimage-x86_64.AppImage" \
    "$APPIMAGE_PLUGIN"
  ln -sf "$APPIMAGE_PLUGIN" "$APPIMAGE_PLUGIN_LINK"

  prepare_appdir
  export PATH="$TOOLS_DIR:$PATH"
  export APPIMAGE_EXTRACT_AND_RUN=1
  export ARCH="$APPIMAGE_ARCH"
  export VERSION
  export OUTPUT="$RELEASE_DIR/$PACKAGE_PREFIX.AppImage"
  export LDAI_OUTPUT="$RELEASE_DIR/$PACKAGE_PREFIX.AppImage"
  export NO_STRIP=1

  "$APPIMAGE_TOOL" \
    --appdir "$APPDIR" \
    -e "$APPDIR/usr/bin/$APP_SLUG" \
    -d "$APPDIR/$APP_SLUG.desktop" \
    -i "$APPDIR/$APP_SLUG.png" \
    --output appimage

  if [ ! -f "$RELEASE_DIR/$PACKAGE_PREFIX.AppImage" ]; then
    echo "AppImage packaging did not produce $RELEASE_DIR/$PACKAGE_PREFIX.AppImage" >&2
    find "$ROOT_DIR" -maxdepth 2 -name '*.AppImage' -print >&2 || true
    exit 1
  fi
}

prepare_workspace
build_tarball
build_deb
build_appimage

echo "Linux release assets created in $RELEASE_DIR"
