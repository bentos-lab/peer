#!/usr/bin/env bash

set -euo pipefail

APP_NAME="peer"
ENTRYPOINT="./cmd/peer"
DIST_DIR="dist"

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)

LDFLAGS="-s -w \
  -X main.version=${VERSION} \
  -X main.commit=${COMMIT}"

echo "🚀 Building ${APP_NAME}"
echo "Version: ${VERSION}"
echo "Commit: ${COMMIT}"
echo ""

# Clean dist
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS="${PLATFORM%/*}"
  GOARCH="${PLATFORM#*/}"

  BIN_NAME="${APP_NAME}"
  ARCHIVE_NAME="${APP_NAME}-${VERSION}-${GOOS}-${GOARCH}"

  if [ "${GOOS}" = "windows" ]; then
    BIN_NAME="${BIN_NAME}.exe"
    ARCHIVE_FILE="${DIST_DIR}/${ARCHIVE_NAME}.zip"
  else
    ARCHIVE_FILE="${DIST_DIR}/${ARCHIVE_NAME}.tar.gz"
  fi

  echo "🔧 Building ${GOOS}/${GOARCH}..."

  TMP_DIR=$(mktemp -d)

  env GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 \
    go build -ldflags="${LDFLAGS}" \
    -o "${TMP_DIR}/${BIN_NAME}" \
    ${ENTRYPOINT}

  if [ "${GOOS}" = "windows" ]; then
    (cd "${TMP_DIR}" && zip -q "${OLDPWD}/${ARCHIVE_FILE}" "${BIN_NAME}")
  else
    (cd "${TMP_DIR}" && tar -czf "${OLDPWD}/${ARCHIVE_FILE}" "${BIN_NAME}")
  fi

  rm -rf "${TMP_DIR}"

  echo "✅ ${ARCHIVE_FILE}"
done

echo ""
echo "🎉 Done! Files are in ./${DIST_DIR}"
