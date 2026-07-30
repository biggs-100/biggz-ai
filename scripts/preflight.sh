#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# ── Check minisign CLI ──────────────────────────────────────────────────────

if ! command -v minisign &>/dev/null; then
    error "minisign CLI not found in PATH"
    echo ""
    echo "  Install it:"
    echo "    macOS: brew install minisign"
    echo "    Linux: apt install minisign  (or: https://github.com/jedisct1/minisign/releases)"
    echo "    Windows: choco install minisign  (or: scoop install minisign)"
    echo ""
    exit 1
fi

info "minisign found at $(command -v minisign)"

# ── Key directory ───────────────────────────────────────────────────────────

KEY_DIR="${HOME}/biggz-release-key"
mkdir -p "${KEY_DIR}"

SECRET_KEY="${KEY_DIR}/minisign.key"
PUBLIC_KEY="${KEY_DIR}/minisign.pub"

if [ ! -f "${SECRET_KEY}" ]; then
    echo ""
    info "No signing key found — generating a new pair in ${KEY_DIR}"
    minisign -G -W -p "${PUBLIC_KEY}" -s "${SECRET_KEY}"
    info "Key pair generated"
else
    info "Existing key pair found in ${KEY_DIR}"
fi

info "Public key:  ${PUBLIC_KEY}"
info "Secret key:  ${SECRET_KEY}"

# ── Sign a test checksum ────────────────────────────────────────────────────

TEST_DIR=$(mktemp -d)
trap 'rm -rf "${TEST_DIR}"' EXIT

TEST_CHECKSUM="${TEST_DIR}/checksums.txt"
echo "deadbeef  test-binary-v1.0.0-linux-amd64.tar.gz" > "${TEST_CHECKSUM}"

info "Signing test checksum file..."
minisign -Sm "${TEST_CHECKSUM}" -x -s "${SECRET_KEY}"

SIGNATURE="${TEST_CHECKSUM}.minisig"
if [ ! -f "${SIGNATURE}" ]; then
    error "Signature file was not created"
    exit 1
fi
info "Signature created: ${SIGNATURE}"

# ── Verify ──────────────────────────────────────────────────────────────────

info "Verifying signature..."
minisign -Vm "${TEST_CHECKSUM}" -P "$(cat "${PUBLIC_KEY}")"

info "All checks passed. Signing pipeline is ready."
echo ""
info "To use in CI:"
echo "  1. Copy the secret key content to a GitHub secret named MINISIGN_PRIVATE_KEY"
echo "     (e.g.: cat ${SECRET_KEY} | clip)"
echo "  2. Commit the public key to your repo root as minisign.pub"
echo "     (already done if this script runs from the repo root)"
echo "  3. Push a v* tag to trigger the release workflow"
