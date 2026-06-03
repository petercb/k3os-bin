#!/usr/bin/env bash
# Downloads k3os-kernel release assets for QEMU integration tests.
# Supports KERNEL_VERSION env var to pin a version (default: latest).
# Caches downloads in integration/qemu/.cache/
# Idempotent: skips if files already exist.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CACHE_DIR="${SCRIPT_DIR}/.cache"
REPO="petercb/k3os-kernel"

# Pin to a known-good kernel version. Override with KERNEL_VERSION env var.
KERNEL_VERSION="${KERNEL_VERSION:-v0.111.0}"

mkdir -p "${CACHE_DIR}"

# Determine the release URL
if [[ "${KERNEL_VERSION}" == "latest" ]]; then
    echo "==> Fetching latest release info from github.com/${REPO}..."
    RELEASE_URL="https://api.github.com/repos/${REPO}/releases/latest"
else
    echo "==> Fetching release info for version ${KERNEL_VERSION} from github.com/${REPO}..."
    RELEASE_URL="https://api.github.com/repos/${REPO}/releases/tags/${KERNEL_VERSION}"
fi

# Fetch release metadata
RELEASE_JSON=$(curl -fsSL "${RELEASE_URL}")
TAG_NAME=$(echo "${RELEASE_JSON}" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | cut -d'"' -f4)

if [[ -z "${TAG_NAME}" ]]; then
    echo "ERROR: Could not determine release tag from GitHub API response."
    exit 1
fi

echo "==> Using release: ${TAG_NAME}"

# Assets to download
ASSETS=("k3os-vmlinuz-amd64.img" "k3os-initrd-amd64.gz")

for asset in "${ASSETS[@]}"; do
    dest="${CACHE_DIR}/${asset}"

    if [[ -f "${dest}" ]]; then
        echo "    [cached] ${asset} already exists, skipping download."
        continue
    fi

    # Extract download URL from release JSON
    download_url=$(echo "${RELEASE_JSON}" | grep -o "\"browser_download_url\"[[:space:]]*:[[:space:]]*\"[^\"]*${asset}\"" | head -1 | cut -d'"' -f4)

    if [[ -z "${download_url}" ]]; then
        echo "ERROR: Could not find download URL for ${asset} in release ${TAG_NAME}."
        exit 1
    fi

    echo "    [download] ${asset} from ${download_url}..."
    curl -fSL --progress-bar -o "${dest}" "${download_url}"

    if [[ ! -s "${dest}" ]]; then
        echo "ERROR: Downloaded file ${dest} is empty or missing."
        rm -f "${dest}"
        exit 1
    fi

    echo "    [done] ${asset} ($(du -h "${dest}" | cut -f1))"
done

echo "==> All kernel assets cached in ${CACHE_DIR}"
