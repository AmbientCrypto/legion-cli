#!/bin/sh
# Install the legion CLI: detects OS and CPU, downloads the release archive from
# GitHub, verifies its sha256 against checksums.txt and installs the binary.
#
#   curl -fsSL https://raw.githubusercontent.com/AmbientCrypto/legion-cli/main/scripts/install.sh | bash
#
# Environment:
#   LEGION_VERSION      version to install, e.g. 1.4.0 (default: latest release)
#   LEGION_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#   LEGION_GITHUB_TOKEN token for a private repository (optional)
set -eu

REPO="AmbientCrypto/legion-cli"
DIR="${LEGION_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*" >&2; }
die() { say "install.sh: $*"; exit 1; }

fetch() { # url dest
  if [ -n "${LEGION_GITHUB_TOKEN:-}" ]; then
    curl --proto '=https' --tlsv1.2 -fsSL -H "Authorization: Bearer ${LEGION_GITHUB_TOKEN}" -o "$2" "$1"
  else
    curl --proto '=https' --tlsv1.2 -fsSL -o "$2" "$1"
  fi
}

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "unsupported OS $(uname -s); on Windows download the .zip from https://github.com/${REPO}/releases" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "unsupported CPU $(uname -m)" ;;
esac

version="${LEGION_VERSION:-}"
if [ -z "$version" ]; then
  final="$(curl --proto '=https' --tlsv1.2 -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")" || die "cannot find the latest release"
  version="${final##*/v}"
fi
[ -n "$version" ] || die "cannot work out the version to install"
# The version ends up in a URL and a file name: letters, digits and . + - only.
printf '%s' "$version" | grep -Eq '^v?[0-9A-Za-z.+-]+$' || die "invalid version: ${version}"
version="${version#v}"

archive="legion_${version}_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/v${version}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "Downloading legion ${version} for ${os}/${arch}..."
fetch "${base}/${archive}" "${tmp}/${archive}" || die "download failed: ${base}/${archive}"
fetch "${base}/checksums.txt" "${tmp}/checksums.txt" || die "download failed: ${base}/checksums.txt"

want="$(awk -v f="$archive" '$2 == f { print $1 }' "${tmp}/checksums.txt")"
[ -n "$want" ] || die "${archive} is not in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "${tmp}/${archive}" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
  got="$(shasum -a 256 "${tmp}/${archive}" | awk '{ print $1 }')"
else
  die "sha256sum or shasum is required to verify the download"
fi
[ "$got" = "$want" ] || die "checksum mismatch for ${archive}: expected ${want}, got ${got}"

tar -xzf "${tmp}/${archive}" -C "$tmp" legion
mkdir -p "$DIR"
install -m 0755 "${tmp}/legion" "${DIR}/legion"
say "Installed ${DIR}/legion ($("${DIR}/legion" version))"

case ":${PATH}:" in
  *":${DIR}:"*) ;;
  *) say "Add ${DIR} to your PATH, for example:  export PATH=\"${DIR}:\$PATH\"" ;;
esac
say "Next: legion auth login"
