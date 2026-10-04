#!/usr/bin/env bash
# End-to-end smoke checks for RUM against a running rum-lab stack.
# Prerequisites: docker compose up in this directory; coroot reachable at COROOT_URL.
# Works with anonymous_role: Admin (default rum-lab) or password login.
set -euo pipefail

COROOT_URL="${COROOT_URL:-http://localhost:8080}"
PROJECT="${PROJECT:-}"
USER="${COROOT_USER:-admin}"
PASS="${COROOT_PASS:-admin}"
# Preconfigured lab key (allowed origins include localhost:3000).
LAB_RUM_KEY="${LAB_RUM_KEY:-rum-local-key-000000000000000001}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

need_jq() { command -v jq >/dev/null || { echo "jq required"; exit 1; }; }

login() {
  # Optional — rum-lab uses anonymous Admin; password login may 404.
  curl -s -c "$COOKIE_JAR" -X POST "$COROOT_URL/api/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$USER\",\"password\":\"$PASS\"}" >/dev/null || true
}

detect_project() {
  if [[ -n "$PROJECT" ]]; then
    echo "using project=$PROJECT"
    return
  fi
  local body
  body=$(curl -sf -b "$COOKIE_JAR" -H 'Accept: application/json' "$COROOT_URL/api/user")
  PROJECT=$(echo "$body" | jq -r '.projects[0].id // empty')
  [[ -n "$PROJECT" ]] || { echo "no project found in /api/user: $body"; exit 1; }
  echo "using project=$PROJECT"
}

api() {
  local method=$1 path=$2
  shift 2
  curl -sf -b "$COOKIE_JAR" -H 'Accept: application/json' -X "$method" "$COROOT_URL$path" "$@"
}

need_jq
login
detect_project

echo "== rum_settings GET =="
settings=$(api GET "/api/project/$PROJECT/rum_settings")
echo "$settings" | jq -e '.editable != null' >/dev/null
echo "$settings" | jq -e '.default_raw_ttl != null and .default_aggregates_ttl != null' >/dev/null
echo "defaults: raw=$(echo "$settings" | jq -r .default_raw_ttl) aggregates=$(echo "$settings" | jq -r .default_aggregates_ttl)"

echo "== generate RUM key without origins (expect fail) =="
code=$(curl -s -o /tmp/rum_key_err.json -w '%{http_code}' -b "$COOKIE_JAR" -X POST \
  "$COROOT_URL/api/project/$PROJECT/api_keys" \
  -H 'Content-Type: application/json' \
  -d '{"action":"generate","description":"e2e-rum","type":"rum","allowed_origins":[]}')
[[ "$code" == "400" ]] || { echo "expected 400 got $code"; cat /tmp/rum_key_err.json; exit 1; }

# Use the preconfigured lab RUM key (project may be config-readonly, so generate can be flaky).
RUM_KEY="$LAB_RUM_KEY"
ORIGIN="http://localhost:3000"
echo "using lab RUM key for ingest"
echo "== OPTIONS preflight =="
curl -sf -X OPTIONS "$COROOT_URL/v1/traces" \
  -H "Origin: $ORIGIN" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: content-type,x-api-key" -o /dev/null

echo "== ingest RUM JSON OTLP =="
now_s=$(date +%s)
now_ns=$((now_s * 1000000000))
end_ns=$((now_ns + 1500000000))
payload=$(cat <<EOF
{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"e2e-web"}},{"key":"telemetry.sdk.language","value":{"stringValue":"webjs"}},{"key":"service.version","value":{"stringValue":"e2e-1"}}]},"scopeSpans":[{"spans":[{"traceId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","spanId":"bbbbbbbbbbbbbbbb","name":"documentLoad","kind":1,"startTimeUnixNano":"${now_ns}","endTimeUnixNano":"${end_ns}","attributes":[{"key":"session.id","value":{"stringValue":"sess-e2e"}},{"key":"page.path","value":{"stringValue":"/product/123"}},{"key":"http.url","value":{"stringValue":"http://localhost:3000/product/123"}}]},{"traceId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","spanId":"cccccccccccccccc","name":"lcp","kind":1,"startTimeUnixNano":"${now_ns}","endTimeUnixNano":"${now_ns}","attributes":[{"key":"session.id","value":{"stringValue":"sess-e2e"}},{"key":"page.path","value":{"stringValue":"/product/123"}},{"key":"vital.value","value":{"stringValue":"1800"}},{"key":"vital.rating","value":{"stringValue":"good"}}]}]}]}]}
EOF
)
curl -sf -X POST "$COROOT_URL/v1/traces" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $RUM_KEY" \
  -H "Origin: $ORIGIN" \
  -H "X-Coroot-Signal: rum" \
  -d "$payload" >/dev/null
echo "ingested documentLoad+lcp for e2e-web"

echo "== RUM key rejected on /v1/logs (security) =="
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$COROOT_URL/v1/logs" \
  -H "Content-Type: application/x-protobuf" \
  -H "X-API-Key: $RUM_KEY" || true)
[[ "$code" == "404" || "$code" == "401" || "$code" == "403" ]] || {
  echo "expected RUM key rejected on /v1/logs, got $code"; exit 1
}
echo "logs status=$code (ok)"

echo "== overview/rum =="
api GET "/api/project/$PROJECT/overview/rum" >/dev/null

editable=$(echo "$settings" | jq -r '.editable')
if [[ "$editable" == "false" ]]; then
  echo "== rum_settings POST (config-readonly project) =="
  code=$(curl -s -o /dev/null -w '%{http_code}' -b "$COOKIE_JAR" -X POST \
    "$COROOT_URL/api/project/$PROJECT/rum_settings" \
    -H 'Content-Type: application/json' \
    -d '{"geo_enabled":false,"replay_enabled":false,"replay_sample_rate":0,"raw_ttl":"1h"}')
  [[ "$code" == "403" ]] || { echo "expected 403 for readonly project got $code"; exit 1; }
  echo "readonly rejected ($code)"
else
  echo "== rum_settings invalid TTL =="
  code=$(curl -s -o /dev/null -w '%{http_code}' -b "$COOKIE_JAR" -X POST \
    "$COROOT_URL/api/project/$PROJECT/rum_settings" \
    -H 'Content-Type: application/json' \
    -d '{"geo_enabled":false,"replay_enabled":false,"replay_sample_rate":0,"raw_ttl":"1h"}')
  [[ "$code" == "400" ]] || { echo "expected 400 for raw_ttl=1h got $code"; exit 1; }
  echo "invalid TTL rejected ($code)"

  echo "== rum_settings save valid TTL =="
  code=$(curl -s -o /tmp/rum_settings_ok.json -w '%{http_code}' -b "$COOKIE_JAR" -X POST \
    "$COROOT_URL/api/project/$PROJECT/rum_settings" \
    -H 'Content-Type: application/json' \
    -d '{"geo_enabled":true,"replay_enabled":false,"replay_sample_rate":0,"raw_ttl":"7d","aggregates_ttl":"30d"}')
  [[ "$code" =~ ^2 ]] || { echo "expected 2xx got $code"; cat /tmp/rum_settings_ok.json; exit 1; }
  echo "save status=$code"
fi

echo "e2e checks finished OK"
