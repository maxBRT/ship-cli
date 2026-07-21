#!/usr/bin/env bash
# Install ship from the latest GitHub Release (GoReleaser assets).
# Usage: curl -fsSL https://raw.githubusercontent.com/maxBRT/ship-cli/main/install.sh | bash
#
# Supports linux/darwin × amd64/arm64. Windows release assets are published on the
# same GitHub Releases page for manual download; this script is Unix-shell-first.
set -euo pipefail

REPO="${SHIP_REPO:-maxBRT/ship-cli}"
API_BASE="${SHIP_GITHUB_API:-https://api.github.com}"
INSTALL_DIR="${SHIP_INSTALL_DIR:-}"

err() {
  printf '%s\n' "$*" >&2
  exit 1
}

detect_platform() {
  local os arch
  os="${SHIP_OS:-$(uname -s)}"
  arch="${SHIP_ARCH:-$(uname -m)}"
  case "$os" in
    Linux|linux) os=linux ;;
    Darwin|darwin) os=darwin ;;
    Windows_NT|windows|MINGW*|MSYS*|CYGWIN*)
      err "unsupported platform: ${os}/${arch} (install script is Unix-first; download Windows assets from https://github.com/${REPO}/releases)"
      ;;
    *) err "unsupported platform: ${os}/${arch} (need linux or darwin with amd64 or arm64)" ;;
  esac
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) err "unsupported platform: ${os}/${arch} (need linux or darwin with amd64 or arm64)" ;;
  esac
  OS="$os"
  ARCH="$arch"
}

resolve_install_dir() {
  if [[ -n "$INSTALL_DIR" ]]; then
    DEST="$INSTALL_DIR"
    return
  fi
  local home_bin="${HOME}/.local/bin"
  if mkdir -p "$home_bin" 2>/dev/null && [[ -w "$home_bin" ]]; then
    DEST="$home_bin"
    return
  fi
  for d in /usr/local/bin /usr/bin; do
    if [[ -d "$d" && -w "$d" ]]; then
      DEST="$d"
      return
    fi
  done
  err "no writable install directory (tried ~/.local/bin, /usr/local/bin, /usr/bin)"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    err "missing required command: sha256sum or shasum"
  fi
}

main() {
  need_cmd curl
  need_cmd tar
  need_cmd mktemp

  detect_platform
  resolve_install_dir
  mkdir -p "$DEST"

  local tmp
  tmp="$(mktemp -d)"
  # shellcheck disable=SC2064
  trap "rm -rf '$tmp'" EXIT

  local release_json tag ver asset sums_name archive sums want got
  release_json="$(curl -fsSL -H 'Accept: application/vnd.github+json' \
    "${API_BASE}/repos/${REPO}/releases/latest")" || err "failed to fetch latest release from ${API_BASE}"

  tag="$(printf '%s' "$release_json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  [[ -n "$tag" ]] || err "latest release missing tag_name"
  ver="${tag#v}"
  asset="ship-cli_${ver}_${OS}_${ARCH}.tar.gz"
  sums_name="ship-cli_${ver}_checksums.txt"

  local asset_url sums_url
  asset_url="$(printf '%s' "$release_json" | sed -n "s/.*\"name\"[[:space:]]*:[[:space:]]*\"${asset}\"[[:space:]]*,[[:space:]]*\"browser_download_url\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" | head -n1)"
  # Prefer parsing assets more carefully with a second pass if sed failed on order
  if [[ -z "$asset_url" ]]; then
    asset_url="$(printf '%s' "$release_json" | tr '{' '\n' | grep -F "\"name\":\"${asset}\"" | sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  fi
  if [[ -z "$asset_url" ]]; then
    asset_url="$(printf '%s' "$release_json" | tr '{' '\n' | grep -F "\"name\": \"${asset}\"" | sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  fi
  [[ -n "$asset_url" ]] || err "no release asset named ${asset} (unsupported platform or incomplete release)"

  sums_url="$(printf '%s' "$release_json" | tr '{' '\n' | grep -F "\"name\":\"${sums_name}\"" | sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [[ -z "$sums_url" ]]; then
    sums_url="$(printf '%s' "$release_json" | tr '{' '\n' | grep -F "\"name\": \"${sums_name}\"" | sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  fi
  [[ -n "$sums_url" ]] || err "no checksums asset named ${sums_name}"

  case "$asset_url" in
    https://*|http://127.*|http://localhost*|http://\[::1\]*) ;;
    http://*) err "download URL must use HTTPS" ;;
    *) ;;
  esac
  case "$sums_url" in
    https://*|http://127.*|http://localhost*|http://\[::1\]*) ;;
    http://*) err "download URL must use HTTPS" ;;
    *) ;;
  esac

  archive="${tmp}/${asset}"
  sums="${tmp}/checksums.txt"
  curl -fsSL "$asset_url" -o "$archive" || err "failed to download ${asset}"
  curl -fsSL "$sums_url" -o "$sums" || err "failed to download checksums"

  want="$(awk -v f="$asset" '$NF == f { print $1; exit }' "$sums")"
  [[ -n "$want" ]] || err "checksums file has no entry for ${asset}"
  got="$(sha256_file "$archive")"
  [[ "$got" == "$want" ]] || err "checksum mismatch for ${asset}"

  tar -xzf "$archive" -C "$tmp"
  [[ -f "${tmp}/ship" ]] || err "archive does not contain ship binary"
  chmod +x "${tmp}/ship"
  mv "${tmp}/ship" "${DEST}/ship"

  printf 'Installed ship %s to %s\n' "$tag" "${DEST}/ship"

  case ":${PATH}:" in
    *":${DEST}:"*) ;;
    *)
      printf 'warning: %s is not on PATH; add it so you can run ship\n' "$DEST" >&2
      ;;
  esac
}

main "$@"
