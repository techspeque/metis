#!/usr/bin/env bash
# Metis installer — downloads the latest release binary for this platform,
# installs it, and ensures it is on PATH.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/techspeque/metis/main/scripts/install.sh | bash
#
# Environment overrides:
#   METIS_VERSION         install a specific version (e.g. v0.0.2); default: latest release
#   METIS_INSTALL_DIR     install directory; default: ~/.local/bin
#   METIS_NO_MODIFY_PATH  set to 1 to skip rc-file PATH setup

set -euo pipefail

REPO="techspeque/metis"
INSTALL_DIR="${METIS_INSTALL_DIR:-$HOME/.local/bin}"

# Temp dir is global so the EXIT trap can still see it after main() returns.
tmp=""
trap '[ -n "$tmp" ] && rm -rf "$tmp"' EXIT

info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
err() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# --- Platform detection ------------------------------------------------------

detect_os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    MINGW* | MSYS* | CYGWIN*)
      err "Windows detected — download the zip from https://github.com/$REPO/releases/latest instead" ;;
    *) err "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo amd64 ;;
    arm64 | aarch64) echo arm64 ;;
    *) err "unsupported architecture: $(uname -m)" ;;
  esac
}

# --- Version resolution ------------------------------------------------------

latest_version() {
  # Parse tag_name from the GitHub API without requiring jq.
  curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
    grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
}

# --- Main --------------------------------------------------------------------

main() {
  command -v curl >/dev/null || err "curl is required"
  command -v tar >/dev/null || err "tar is required"

  local os arch tag version
  os=$(detect_os)
  arch=$(detect_arch)

  tag="${METIS_VERSION:-$(latest_version)}"
  [ -n "$tag" ] || err "could not determine the latest release (set METIS_VERSION to pin one)"
  version="${tag#v}"

  local archive="metis_${version}_${os}_${arch}.tar.gz"
  local base_url="https://github.com/$REPO/releases/download/$tag"

  info "Installing metis $tag ($os/$arch) to $INSTALL_DIR"

  tmp=$(mktemp -d)

  curl -fsSL -o "$tmp/$archive" "$base_url/$archive" ||
    err "download failed: $base_url/$archive"

  # Verify the checksum when a sha256 tool is available.
  if command -v sha256sum >/dev/null || command -v shasum >/dev/null; then
    curl -fsSL -o "$tmp/checksums.txt" "$base_url/checksums.txt" ||
      err "download failed: $base_url/checksums.txt"
    local expected actual
    expected=$(grep " $archive\$" "$tmp/checksums.txt" | awk '{print $1}')
    [ -n "$expected" ] || err "no checksum for $archive in checksums.txt"
    if command -v sha256sum >/dev/null; then
      actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
    else
      actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
    fi
    [ "$expected" = "$actual" ] || err "checksum mismatch for $archive"
    info "Checksum verified"
  else
    info "Skipping checksum verification (no sha256sum/shasum found)"
  fi

  tar -xzf "$tmp/$archive" -C "$tmp" metis
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$tmp/metis" "$INSTALL_DIR/metis"

  info "Installed: $("$INSTALL_DIR/metis" --version)"

  ensure_path
}

# --- PATH setup --------------------------------------------------------------

# Appends INSTALL_DIR to PATH in the rc file matching the user's login shell.
# Idempotent: skips when the dir is already on PATH or the rc file already
# contains the line we would add.
ensure_path() {
  if [ "${METIS_NO_MODIFY_PATH:-0}" = "1" ]; then
    return
  fi

  case ":$PATH:" in
    *":$INSTALL_DIR:"*)
      return ;;
  esac

  local shell_name rc_file path_line
  shell_name=$(basename "${SHELL:-sh}")

  case "$shell_name" in
    zsh)
      rc_file="$HOME/.zshrc"
      path_line="export PATH=\"$INSTALL_DIR:\$PATH\""
      ;;
    bash)
      # macOS bash reads .bash_profile for login shells; prefer it when present.
      if [ "$(uname -s)" = "Darwin" ] && [ -f "$HOME/.bash_profile" ]; then
        rc_file="$HOME/.bash_profile"
      else
        rc_file="$HOME/.bashrc"
      fi
      path_line="export PATH=\"$INSTALL_DIR:\$PATH\""
      ;;
    fish)
      rc_file="$HOME/.config/fish/config.fish"
      path_line="fish_add_path $INSTALL_DIR"
      ;;
    *)
      info "$INSTALL_DIR is not on your PATH — add it to your shell profile:"
      info "  export PATH=\"$INSTALL_DIR:\$PATH\""
      return
      ;;
  esac

  if [ -f "$rc_file" ] && grep -qF "$path_line" "$rc_file"; then
    info "$rc_file already configures PATH — restart your shell to pick it up"
    return
  fi

  mkdir -p "$(dirname "$rc_file")"
  {
    printf '\n# Added by the metis installer\n'
    printf '%s\n' "$path_line"
  } >>"$rc_file"

  info "Added $INSTALL_DIR to PATH in $rc_file"
  info "Restart your shell (or 'source $rc_file') to use metis"
}

main "$@"
