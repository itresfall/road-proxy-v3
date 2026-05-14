#!/usr/bin/env bash
set -euo pipefail

DRY_RUN="0"
PORTS=("8080/tcp" "8081/tcp")

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN="1"
      shift
      ;;
    --with-http)
      PORTS+=("80/tcp" "443/tcp")
      shift
      ;;
    --port)
      if [[ $# -lt 2 ]]; then
        echo "--port requires a value like 7777/udp or 8080/tcp" >&2
        exit 1
      fi
      PORTS+=("$2")
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if ! command -v ufw >/dev/null 2>&1; then
  echo "ufw not found. Install ufw or open the ports with your provider firewall." >&2
  exit 1
fi

for port in "${PORTS[@]}"; do
  if [[ "${DRY_RUN}" == "1" ]]; then
    echo "Would run: sudo ufw allow ${port}"
  else
    sudo ufw allow "${port}"
  fi
done
