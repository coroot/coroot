---
name: coroot-rebase-upstream
description: Rebase ветки add_mongodb_tls-2 на upstream coroot тэг с проверкой MongoDB TLS поддержки, сборкой и публикацией Docker-образа
---

# coroot-rebase-upstream — Синхронизация MongoDB TLS форка с upstream

## Overview

This skill synchronizes the fork branch `add_mongodb_tls-2` with an upstream `coroot/coroot` release tag.

Use it when you need to:

- rebase `add_mongodb_tls-2` onto a specific upstream tag
- check whether upstream already has MongoDB TLS support before rebasing
- build and publish the image `ghcr.io/tribunadigital/coroot`

Required input:

- `TAG` — upstream tag, for example `v1.19.0`

Hardcoded constants used by this skill:

- Upstream URL: `https://github.com/coroot/coroot.git`
- Upstream remote name: `upstream`
- Fork branch: `add_mongodb_tls-2`
- Docker registry: `ghcr.io/tribunadigital/coroot`
- Dockerfile: `dev.dockerfile`

## Prerequisites

Before starting, verify all of the following:

```bash
# Check tools available
git --version || { echo "ERROR: git not found"; exit 1; }
docker --version || { echo "ERROR: docker not found"; exit 1; }

# Check clean working directory
if [ -n "$(git status --porcelain)" ]; then
  echo "ERROR: Working directory has uncommitted changes. Commit or stash before proceeding."
  git status --short
  # STOP
fi

# Verify correct repository
ORIGIN=$(git remote get-url origin 2>/dev/null)
if ! echo "$ORIGIN" | grep -q "tribunadigital/coroot"; then
  echo "ERROR: Not in tribunadigital/coroot repo. Current origin: $ORIGIN"
  # STOP
fi
```

Additional checks:

- confirm the current shell can reach `github.com`
- confirm the `origin` remote exists
- confirm the user is willing to rebase `add_mongodb_tls-2`

If any prerequisite fails, stop immediately.

## Workflow

### Step 1: Configure Upstream Remote

```bash
# Check if upstream remote exists
UPSTREAM_URL=$(git remote get-url upstream 2>/dev/null)
if [ -z "$UPSTREAM_URL" ]; then
  git remote add upstream https://github.com/coroot/coroot.git
  echo "Upstream remote added"
else
  git remote set-url upstream https://github.com/coroot/coroot.git
  echo "Upstream remote URL updated"
fi
```

If this command fails, report the error and STOP.

### Step 2: Fetch Upstream Tags

```bash
git fetch upstream --tags
```

If fetch fails, assume a network or access problem and STOP.

### Step 3: Validate Tag Exists

```bash
git rev-parse --verify "refs/tags/${TAG}" >/dev/null 2>&1
```

If the tag is missing, show the latest tags:

```bash
git tag --sort=-version:refname | head -20
```

Then STOP.

### Step 4: Extract Version Number

```bash
VERSION=$(echo "${TAG}" | sed 's/^v//')
# v1.19.0 → 1.19.0
```

Use this version for the Docker image tag.

### Step 5: Detect MongoDB TLS in Upstream

Check 3 indicators in the TARGET TAG's tree, not the current branch:

```bash
# Indicator 1: TLS annotation in MongoDB docs
INDICATOR_1=$(git show ${TAG}:docs/docs/databases/mongodb.md 2>/dev/null | grep -i "mongodb-scrape-param-tls")

# Indicator 2: SNI/TLS field in collector config
INDICATOR_2=$(git show ${TAG}:collector/config.go 2>/dev/null | grep -iE "Sni|MongoTls|mongo.*tls")

# Indicator 3: TLS UI element in ApplicationInstrumentation
INDICATOR_3=$(git show ${TAG}:front/src/components/ApplicationInstrumentation.vue 2>/dev/null | grep -iE "mongodb.*tls|mongo.*tls")

# Count non-empty indicators
COUNT=0
[ -n "$INDICATOR_1" ] && COUNT=$((COUNT+1))
[ -n "$INDICATOR_2" ] && COUNT=$((COUNT+1))
[ -n "$INDICATOR_3" ] && COUNT=$((COUNT+1))
```

Decision logic:

- If `COUNT >= 2` → upstream HAS MongoDB TLS support
  - show the user what was found
  - explain how upstream implemented it
  - ask: `Upstream добавил поддержку MongoDB TLS в ${TAG}. Что делаем дальше? 1) Продолжить rebase (наша реализация может отличаться) 2) Остановиться — проанализировать различия 3) Отменить — upstream покрывает наши потребности`
  - STOP — wait for user response
- If `COUNT < 2` → upstream does NOT support MongoDB TLS, continue

### Step 6: Checkout Fork Branch

```bash
git checkout add_mongodb_tls-2
```

If the branch is not local, try:

```bash
git checkout -b add_mongodb_tls-2 origin/add_mongodb_tls-2
```

If that also fails, STOP and report the branch state.

### Step 7: Create Backup Ref

```bash
git branch add_mongodb_tls-2-pre-rebase-${TAG} add_mongodb_tls-2
echo "Backup created: add_mongodb_tls-2-pre-rebase-${TAG}"
```

If backup creation fails, STOP.

### Step 8: Rebase onto Upstream Tag

```bash
git rebase ${TAG}
```

On conflict:

```bash
git rebase --abort
```

Show conflicting files, explain the conflict, and STOP. Do not auto-resolve.

### Step 9: Build Docker Image

```bash
docker build --progress=plain \
  --build-arg VERSION=v${VERSION}-mongo-tls \
  -f dev.dockerfile \
  -t ghcr.io/tribunadigital/coroot:${VERSION}-mongo-tls-2 .
```

If build fails, show the full error output and STOP.

### Step 10: Push Branch and Image

Before pushing, ask for confirmation:

```text
Готово к публикации:
  - git push --force-with-lease origin add_mongodb_tls-2
  - docker push ghcr.io/tribunadigital/coroot:${VERSION}-mongo-tls-2
Продолжить?
```

After confirmation, run:

```bash
git push --force-with-lease origin add_mongodb_tls-2
docker push ghcr.io/tribunadigital/coroot:${VERSION}-mongo-tls-2
```

Use `--force-with-lease`, never `--force`.

If git push fails, STOP.

If docker push fails, STOP and report that the branch is already pushed.

### Step 11: Success Report

```text
✅ Синхронизация завершена успешно!

Ветка: add_mongodb_tls-2 (rebased на ${TAG})
Backup: add_mongodb_tls-2-pre-rebase-${TAG}
Docker image: ghcr.io/tribunadigital/coroot:${VERSION}-mongo-tls-2

Для отката: git push --force-with-lease origin add_mongodb_tls-2-pre-rebase-${TAG}:add_mongodb_tls-2
```

The final report should be short and include the backup ref.

## Error Handling

| Ситуация | Действие |
|----------|----------|
| Dirty working directory | Сообщить о незакоммиченных изменениях, STOP |
| Upstream fetch failed | Проверить сеть/доступ к github.com, STOP |
| Тэг не существует | Показать доступные тэги (`git tag --sort=-version:refname \| head -20`), STOP |
| Upstream имеет MongoDB TLS (≥2 индикатора) | Показать найденный код, объяснить реализацию, спросить пользователя, STOP |
| Rebase конфликт | `git rebase --abort`, показать конфликтные файлы, STOP (НЕ авто-разрешать) |
| Docker build failed | Показать полный вывод ошибки, STOP (ветка rebased, не pushed) |
| Git push failed | Сообщить ошибку, STOP |
| Docker push failed | Сообщить ошибку, STOP (ветка уже pushed) |

Every error case must end with a terminal action: STOP or остановиться.

## Quick Reference

```text
prereqs → remote → fetch → validate tag → detect TLS → checkout → backup → rebase → build → push → report
```

Notes:

- The TLS check uses 3 indicators and the explicit threshold is `COUNT >= 2`.
- The upstream repository is `coroot/coroot`.
- The fork branch is always `add_mongodb_tls-2`.
- The Docker registry is always `ghcr.io/tribunadigital/coroot`.
- The Dockerfile is always `dev.dockerfile`.
- The branch push must use `force-with-lease`.

Operational reminders:

- Never touch other branches.
- Never auto-resolve conflicts.
- Never publish the image before the rebase is validated.
- Never use `--force`.

### Input checklist

- `TAG` provided by the user
- user confirms publication in Step 10
- repository is clean
- upstream remote is reachable

### Output checklist

- rebased branch
- backup ref created
- docker image built
- branch and image published after confirmation

### Minimal user prompt

```text
Enter upstream tag (for example v1.19.0), then follow the workflow exactly.
```
