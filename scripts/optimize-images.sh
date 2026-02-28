#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-web/images}"

if [ ! -d "$TARGET_DIR" ]; then
  echo "target directory not found: $TARGET_DIR" >&2
  exit 1
fi

if command -v pngquant >/dev/null 2>&1; then
  find "$TARGET_DIR" -type f -name '*.png' -print0 | while IFS= read -r -d '' file; do
    pngquant --force --skip-if-larger --ext .png "$file"
  done
else
  echo "pngquant not installed; skipping PNG optimization"
fi

if command -v jpegoptim >/dev/null 2>&1; then
  find "$TARGET_DIR" -type f \( -name '*.jpg' -o -name '*.jpeg' \) -print0 | while IFS= read -r -d '' file; do
    jpegoptim --strip-all "$file"
  done
else
  echo "jpegoptim not installed; skipping JPEG optimization"
fi

echo "image optimization completed for $TARGET_DIR"
