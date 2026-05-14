#!/usr/bin/env bash
set -euo pipefail

ARCH="${1:-auto}"
OUTPUT_ROOT="${2:-build/linux}"

detect_arch() {
  local uname_arch
  uname_arch="$(uname -m | tr '[:upper:]' '[:lower:]')"
  case "${uname_arch}" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    *)
      echo ""
      ;;
  esac
}

if [[ "${ARCH}" == "auto" ]]; then
  ARCH="$(detect_arch)"
  if [[ -z "${ARCH}" ]]; then
    echo "Could not auto-detect linux arch from uname -m. Use: ./scripts/build-linux.sh amd64|arm64" >&2
    exit 1
  fi
fi

if [[ "${ARCH}" != "amd64" && "${ARCH}" != "arm64" ]]; then
  echo "Unsupported arch: ${ARCH}. Allowed: amd64, arm64, auto" >&2
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${REPO_ROOT}/${OUTPUT_ROOT}/${ARCH}"

VERSION="${ROAD_VERSION:-0.1.0-dev}"
COMMIT="${ROAD_COMMIT:-$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || true)}"
COMMIT="${COMMIT:-unknown}"
BUILD_DATE="${ROAD_BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-X road-proxy-v3/internal/version.Version=${VERSION} -X road-proxy-v3/internal/version.Commit=${COMMIT} -X road-proxy-v3/internal/version.BuildDate=${BUILD_DATE}"

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

build_one() {
  local name="$1"
  local pkg="$2"
  local out_path="${OUT_DIR}/${name}"

  echo "Building linux/${ARCH} -> ${name}"
  GOOS=linux GOARCH="${ARCH}" CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o "${out_path}" "${pkg}"
  chmod +x "${out_path}"
}

pushd "${REPO_ROOT}" >/dev/null
build_one "road-proxy" "./cmd/road"
build_one "road-server" "./cmd/server"
build_one "road-client" "./cmd/client"
build_one "plugin-studio" "./cmd/plugin-studio"

rm -rf "${OUT_DIR}/configs" "${OUT_DIR}/plugins" "${OUT_DIR}/locales" "${OUT_DIR}/docs" "${OUT_DIR}/compat-profiles" "${OUT_DIR}/deploy"
cp -R "${REPO_ROOT}/configs" "${OUT_DIR}/configs"
cp -R "${REPO_ROOT}/plugins" "${OUT_DIR}/plugins"
cp -R "${REPO_ROOT}/locales" "${OUT_DIR}/locales"
cp -R "${REPO_ROOT}/docs" "${OUT_DIR}/docs"
cp -R "${REPO_ROOT}/compat-profiles" "${OUT_DIR}/compat-profiles"
cp -R "${REPO_ROOT}/deploy" "${OUT_DIR}/deploy"
cp -f "${REPO_ROOT}/README.md" "${OUT_DIR}/README.md"
cp -f "${REPO_ROOT}/CHANGELOG.md" "${OUT_DIR}/CHANGELOG.md"
cp -f "${REPO_ROOT}/LICENSE" "${OUT_DIR}/LICENSE"
cp -f "${REPO_ROOT}/SECURITY.md" "${OUT_DIR}/SECURITY.md"
cp -f "${REPO_ROOT}/CONTRIBUTING.md" "${OUT_DIR}/CONTRIBUTING.md"
chmod +x "${OUT_DIR}/deploy/linux/firewall-ufw.sh" 2>/dev/null || true
popd >/dev/null

if [[ "${ARCH}" == "amd64" ]]; then
  echo ""
  echo "Refreshing legacy linux paths under ${REPO_ROOT}/${OUTPUT_ROOT}"
  rm -f "${REPO_ROOT}/${OUTPUT_ROOT}/road-proxy" \
    "${REPO_ROOT}/${OUTPUT_ROOT}/road-server" \
    "${REPO_ROOT}/${OUTPUT_ROOT}/road-client" \
    "${REPO_ROOT}/${OUTPUT_ROOT}/plugin-studio"
  cp -f "${OUT_DIR}/road-proxy" "${REPO_ROOT}/${OUTPUT_ROOT}/road-proxy"
  cp -f "${OUT_DIR}/road-server" "${REPO_ROOT}/${OUTPUT_ROOT}/road-server"
  cp -f "${OUT_DIR}/road-client" "${REPO_ROOT}/${OUTPUT_ROOT}/road-client"
  cp -f "${OUT_DIR}/plugin-studio" "${REPO_ROOT}/${OUTPUT_ROOT}/plugin-studio"
  rm -rf "${REPO_ROOT}/${OUTPUT_ROOT}/configs" "${REPO_ROOT}/${OUTPUT_ROOT}/plugins" "${REPO_ROOT}/${OUTPUT_ROOT}/locales" "${REPO_ROOT}/${OUTPUT_ROOT}/docs" "${REPO_ROOT}/${OUTPUT_ROOT}/compat-profiles" "${REPO_ROOT}/${OUTPUT_ROOT}/deploy"
  cp -R "${OUT_DIR}/configs" "${REPO_ROOT}/${OUTPUT_ROOT}/configs"
  cp -R "${OUT_DIR}/plugins" "${REPO_ROOT}/${OUTPUT_ROOT}/plugins"
  cp -R "${OUT_DIR}/locales" "${REPO_ROOT}/${OUTPUT_ROOT}/locales"
  cp -R "${OUT_DIR}/docs" "${REPO_ROOT}/${OUTPUT_ROOT}/docs"
  cp -R "${OUT_DIR}/compat-profiles" "${REPO_ROOT}/${OUTPUT_ROOT}/compat-profiles"
  cp -R "${OUT_DIR}/deploy" "${REPO_ROOT}/${OUTPUT_ROOT}/deploy"
  cp -f "${OUT_DIR}/README.md" "${REPO_ROOT}/${OUTPUT_ROOT}/README.md"
  cp -f "${OUT_DIR}/CHANGELOG.md" "${REPO_ROOT}/${OUTPUT_ROOT}/CHANGELOG.md"
  cp -f "${OUT_DIR}/LICENSE" "${REPO_ROOT}/${OUTPUT_ROOT}/LICENSE"
  cp -f "${OUT_DIR}/SECURITY.md" "${REPO_ROOT}/${OUTPUT_ROOT}/SECURITY.md"
  cp -f "${OUT_DIR}/CONTRIBUTING.md" "${REPO_ROOT}/${OUTPUT_ROOT}/CONTRIBUTING.md"
  chmod +x "${REPO_ROOT}/${OUTPUT_ROOT}/deploy/linux/firewall-ufw.sh" 2>/dev/null || true
  chmod +x "${REPO_ROOT}/${OUTPUT_ROOT}/road-proxy" \
    "${REPO_ROOT}/${OUTPUT_ROOT}/road-server" \
    "${REPO_ROOT}/${OUTPUT_ROOT}/road-client" \
    "${REPO_ROOT}/${OUTPUT_ROOT}/plugin-studio"
fi

echo ""
echo "Linux build complete: ${OUT_DIR}"
echo "Tip: verify with 'file ${OUT_DIR}/road-proxy'"
