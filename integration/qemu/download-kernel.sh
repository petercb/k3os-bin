#!/usr/bin/env bash
# Downloads k3os-kernel release assets for QEMU integration tests.
# Requires KERNEL_VERSION env var (set by the Makefile).
# Caches downloads in integration/qemu/.cache/
# Idempotent: skips if files already exist.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CACHE_DIR="${SCRIPT_DIR}/.cache"
REPO="petercb/k3os-kernel"

# KERNEL_VERSION must be provided by the caller (Makefile or env).
KERNEL_VERSION="${KERNEL_VERSION:-}"
if [[ -z "${KERNEL_VERSION}" ]]; then
    echo "ERROR: KERNEL_VERSION must be set."
    exit 1
fi

# Build optional auth header for GitHub API requests.
CURL_AUTH=()
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    CURL_AUTH=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

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
RELEASE_JSON=$(curl -fsSL "${CURL_AUTH[@]}" "${RELEASE_URL}")
TAG_NAME=$(echo "${RELEASE_JSON}" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | cut -d'"' -f4)

if [[ -z "${TAG_NAME}" ]]; then
    echo "ERROR: Could not determine release tag from GitHub API response."
    exit 1
fi

echo "==> Using release: ${TAG_NAME}"

# Assets to download
ASSETS=("k3os-vmlinuz-amd64.img" "k3os-modules-amd64.tar.gz")

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
