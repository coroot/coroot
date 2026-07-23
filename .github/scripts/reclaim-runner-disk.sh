#!/usr/bin/env bash
# Reclaim disk on self-hosted runners before build/test jobs.
# Safe to run at job start: skips _work dirs for runners with active jobs.
set -uo pipefail

echo "=== disk before ==="
df -h / 2>/dev/null || df -h

docker image prune -af 2>/dev/null || true
docker container prune -f 2>/dev/null || true
docker volume prune -f 2>/dev/null || true
docker network prune -f 2>/dev/null || true

usage=$(df / 2>/dev/null | tail -1 | awk '{print $5}' | tr -dc '0-9' || echo 0)
if [ "${usage:-0}" -ge 70 ]; then
  echo "disk at ${usage}% — aggressive cleanup"
  docker builder prune -af 2>/dev/null || true
  for b in $(docker buildx ls --format '{{.Name}}' 2>/dev/null | sort -u); do
    docker buildx prune -af --builder "$b" 2>/dev/null || true
  done

  workers=""
  uncertain=0
  for pid in $(pgrep -f 'Runner\.Worker' 2>/dev/null); do
    cwd=$(readlink "/proc/$pid/cwd" 2>/dev/null || true)
    if [ -n "$cwd" ]; then
      workers="$workers|$cwd"
    else
      uncertain=1
    fi
  done

  if [ "$uncertain" -eq 0 ]; then
    for root in "$HOME"/actions-runner-*; do
      work="$root/_work"
      [ -d "$work" ] || continue
      busy=0
      IFS='|' read -ra ws <<< "$workers"
      for w in "${ws[@]}"; do
        [ -n "$w" ] || continue
        case "$w" in "$root"*) busy=1; break;; esac
      done
      if [ "$busy" -eq 0 ]; then
        rm -rf "$work"/* 2>/dev/null || true
        echo "cleared stale _work in $root"
      fi
    done
  fi
fi

echo "=== disk after ==="
df -h / 2>/dev/null || df -h
