#!/usr/bin/env python3
"""
Synthetic traffic for the RUM lab: emits browser-like RUM spans and hits backends
(with matching traceparent). Drives two sites (shop + portal) with realistic CWV.
"""
from __future__ import annotations

import json
import os
import random
import time
import urllib.error
import urllib.request
import uuid
from dataclasses import dataclass
from typing import Any

INTERVAL_SEC = float(os.environ.get("INTERVAL_SEC", "5"))
JITTER_SEC = float(os.environ.get("JITTER_SEC", "2"))
ERROR_RATE = float(os.environ.get("ERROR_RATE", "0.25"))
SLOW_RATE = float(os.environ.get("SLOW_RATE", "0.15"))

COROOT_OTLP = os.environ.get("COROOT_OTLP_ENDPOINT", "http://coroot:8080/v1/traces").rstrip("/")
if not COROOT_OTLP.endswith("/v1/traces"):
    COROOT_OTLP = COROOT_OTLP.rstrip("/") + "/v1/traces"

RUM_API_KEY = os.environ.get("RUM_API_KEY", "rum-local-key-000000000000000001")
RUM_VERSION = os.environ.get("RUM_SERVICE_VERSION", "1.0.0")

BROWSERS = [
    ("Chrome", "macOS", "desktop"),
    ("Chrome", "Windows", "desktop"),
    ("Safari", "iOS", "mobile"),
    ("Firefox", "Linux", "desktop"),
    ("Chrome", "Android", "mobile"),
]


@dataclass
class Site:
    name: str
    weight: float
    origin: str
    service: str
    version: str
    pages: list[str]
    kind: str  # shop | portal
    api_base: str = ""
    api_host: str = ""
    api_peer: str = ""
    api_public: str = ""
    checkout_base: str = ""
    checkout_host: str = ""
    checkout_peer: str = ""
    checkout_public: str = ""

    def __post_init__(self) -> None:
        if self.api_host and not self.api_public:
            self.api_public = f"http://{self.api_host}"
        if self.checkout_host and not self.checkout_public:
            self.checkout_public = f"http://{self.checkout_host}"


def _env_site_shop() -> Site:
    api_host = os.environ.get("API_HOST", "localhost:4000")
    checkout_host = os.environ.get("CHECKOUT_HOST", "localhost:4001")
    return Site(
        name="shop",
        weight=float(os.environ.get("SHOP_WEIGHT", "0.6")),
        origin=os.environ.get("RUM_ORIGIN", "http://localhost:3000"),
        service=os.environ.get("RUM_SERVICE_NAME", "demo-web"),
        version=os.environ.get("RUM_SERVICE_VERSION", "1.0.0"),
        pages=["/", "/shop.html", "/checkout.html"],
        kind="shop",
        api_base=os.environ.get("API_BASE", "http://demo-api:4000").rstrip("/"),
        api_host=api_host,
        api_peer=os.environ.get("PEER_SERVICE", "demo-api"),
        api_public=os.environ.get("API_PUBLIC_BASE", f"http://{api_host}").rstrip("/"),
        checkout_base=os.environ.get("CHECKOUT_BASE", "http://demo-checkout:4001").rstrip("/"),
        checkout_host=checkout_host,
        checkout_peer=os.environ.get("CHECKOUT_PEER_SERVICE", "demo-checkout"),
        checkout_public=os.environ.get("CHECKOUT_PUBLIC_BASE", f"http://{checkout_host}").rstrip("/"),
    )


def _env_site_portal() -> Site:
    api_host = os.environ.get("PORTAL_API_HOST", "localhost:4002")
    return Site(
        name="portal",
        weight=float(os.environ.get("PORTAL_WEIGHT", "0.4")),
        origin=os.environ.get("PORTAL_RUM_ORIGIN", "http://localhost:3001"),
        service=os.environ.get("PORTAL_RUM_SERVICE_NAME", "demo-portal"),
        version=os.environ.get("PORTAL_RUM_SERVICE_VERSION", "1.0.0"),
        pages=["/", "/tickets.html", "/status.html"],
        kind="portal",
        api_base=os.environ.get("PORTAL_API_BASE", "http://demo-portal-api:4002").rstrip("/"),
        api_host=api_host,
        api_peer=os.environ.get("PORTAL_PEER_SERVICE", "demo-portal-api"),
        api_public=os.environ.get("PORTAL_API_PUBLIC_BASE", f"http://{api_host}").rstrip("/"),
    )


SITES = [_env_site_shop(), _env_site_portal()]


def pick_site() -> Site:
    total = sum(s.weight for s in SITES) or 1.0
    r = random.random() * total
    acc = 0.0
    for s in SITES:
        acc += s.weight
        if r <= acc:
            return s
    return SITES[0]


def new_id(n: int) -> str:
    return uuid.uuid4().hex[:n]


def now_ns() -> int:
    return time.time_ns()


def attr_str(key: str, value: Any) -> dict[str, Any]:
    return {"key": key, "value": {"stringValue": str(value)}}


def attr_int(key: str, value: int) -> dict[str, Any]:
    return {"key": key, "value": {"intValue": str(int(value))}}


def attr_double(key: str, value: float) -> dict[str, Any]:
    return {"key": key, "value": {"doubleValue": float(value)}}


def post_json(url: str, body: dict[str, Any], headers: dict[str, str], timeout: float = 15) -> tuple[int, bytes]:
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, method="POST", headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read() if e.fp else b""
    except Exception as e:
        print(f"[traffic] POST {url} failed: {e}", flush=True)
        return 0, b""


def get_json(url: str, headers: dict[str, str] | None = None, timeout: float = 15) -> tuple[int, Any]:
    req = urllib.request.Request(url, method="GET", headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            try:
                return resp.status, json.loads(raw.decode("utf-8") or "{}")
            except json.JSONDecodeError:
                return resp.status, {}
    except urllib.error.HTTPError as e:
        raw = e.read() if e.fp else b""
        try:
            return e.code, json.loads(raw.decode("utf-8") or "{}")
        except Exception:
            return e.code, {}
    except Exception as e:
        print(f"[traffic] GET {url} failed: {e}", flush=True)
        return 0, {}


def make_span(
    *,
    name: str,
    kind: int,
    trace_id: str,
    span_id: str,
    parent_span_id: str,
    start_ns: int,
    end_ns: int,
    attributes: list[dict[str, Any]],
    status_code: int = 1,
    status_message: str = "",
) -> dict[str, Any]:
    span: dict[str, Any] = {
        "traceId": trace_id,
        "spanId": span_id,
        "name": name,
        "kind": kind,
        "startTimeUnixNano": str(start_ns),
        "endTimeUnixNano": str(end_ns),
        "attributes": attributes,
        "status": {"code": status_code, "message": status_message},
    }
    if parent_span_id:
        span["parentSpanId"] = parent_span_id
    return span


def rate_vital(name: str, value: float) -> str:
    if name in ("lcp", "fcp"):
        return "good" if value <= 2500 else "needs-improvement" if value <= 4000 else "poor"
    if name == "inp":
        return "good" if value <= 200 else "needs-improvement" if value <= 500 else "poor"
    if name == "cls":
        return "good" if value <= 0.1 else "needs-improvement" if value <= 0.25 else "poor"
    if name == "ttfb":
        return "good" if value <= 800 else "needs-improvement" if value <= 1800 else "poor"
    return "good"


def sample_bucket(good: tuple[float, float], ni: tuple[float, float], poor: tuple[float, float], slow: bool = False) -> float:
    """~70/20/10 good/NI/poor; slow scenarios bias toward NI/poor."""
    r = random.random()
    if slow:
        if r < 0.25:
            lo, hi = good
        elif r < 0.65:
            lo, hi = ni
        else:
            lo, hi = poor
    else:
        if r < 0.70:
            lo, hi = good
        elif r < 0.90:
            lo, hi = ni
        else:
            lo, hi = poor
    return random.uniform(lo, hi)


def sample_vitals(*, slow: bool = False, backend_delay_ms: float = 0) -> dict[str, float]:
    lcp = sample_bucket((800, 2200), (2500, 3800), (4000, 8000), slow=slow)
    ttfb = sample_bucket((40, 400), (800, 1400), (1800, 3500), slow=slow)
    inp = sample_bucket((40, 180), (200, 450), (500, 1200), slow=slow)
    cls = sample_bucket((0.02, 0.08), (0.10, 0.22), (0.25, 0.35), slow=slow)
    if backend_delay_ms > 0:
        # Correlate TTFB/LCP with measured backend latency.
        ttfb = max(ttfb, backend_delay_ms * 0.85 + random.uniform(20, 120))
        lcp = max(lcp, ttfb + random.uniform(200, 1200))
    return {"lcp": lcp, "ttfb": ttfb, "inp": inp, "cls": cls}


def export_rum(
    site: Site,
    spans: list[dict[str, Any]],
    hint: str = "",
    browser: tuple[str, str, str] | None = None,
) -> None:
    browser = browser or random.choice(BROWSERS)
    body = {
        "resourceSpans": [
            {
                "resource": {
                    "attributes": [
                        attr_str("service.name", site.service),
                        attr_str("service.version", site.version or RUM_VERSION),
                        attr_str("telemetry.sdk.language", "webjs"),
                        attr_str("telemetry.sdk.name", "coroot-rum"),
                        attr_str("browser.name", browser[0]),
                        attr_str("os.name", browser[1]),
                        attr_str("device.type", browser[2]),
                        attr_str("session.id", new_id(16)),
                        attr_str("deployment.environment", "rum-lab"),
                    ]
                },
                "scopeSpans": [{"scope": {"name": "coroot-rum", "version": "0.2.0"}, "spans": spans}],
            }
        ]
    }
    headers = {
        "Content-Type": "application/json",
        "X-API-Key": RUM_API_KEY,
        "X-Coroot-Signal": "rum",
        "Origin": site.origin,
    }
    if hint:
        headers["X-Coroot-Rum-Hint"] = hint
    code, _ = post_json(COROOT_OTLP, body, headers)
    if code >= 300:
        print(f"[traffic] RUM export HTTP {code} site={site.service}", flush=True)


def add_vitals(
    rum_spans: list[dict[str, Any]],
    trace_id: str,
    parent_id: str,
    page: str,
    after_ns: int,
    *,
    slow: bool = False,
    backend_delay_ms: float = 0,
) -> None:
    vals = sample_vitals(slow=slow, backend_delay_ms=backend_delay_ms)
    for vital, value in vals.items():
        if vital == "cls" and random.random() > 0.85:
            continue
        rating = rate_vital(vital, value)
        s = after_ns + int(random.uniform(5, 60) * 1e6)
        # Span duration for CLS uses a small synthetic window; value is the CLS score.
        dur_for_span = value if vital != "cls" else max(1.0, value * 1000)
        e = s + int(max(dur_for_span, 1) * 1e6)
        rum_spans.append(
            make_span(
                name=vital,
                kind=1,
                trace_id=trace_id,
                span_id=new_id(16),
                parent_span_id=parent_id,
                start_ns=s,
                end_ns=e,
                attributes=[
                    attr_str("page.path", page),
                    attr_str("event.type", vital),
                    attr_str("vital.name", vital),
                    attr_double("vital.value", value),
                    attr_str("vital.rating", rating),
                    attr_double("value", value),
                ],
            )
        )


def http_client_span(
    *,
    site: Site,
    rum_spans: list[dict[str, Any]],
    method: str,
    public_url: str,
    host: str,
    peer: str,
    status: int,
    trace_id: str,
    span_id: str,
    parent_id: str,
    start_ns: int,
    end_ns: int,
    page: str = "",
    extra_attrs: list[dict[str, Any]] | None = None,
) -> None:
    ok = 200 <= status < 400
    attrs = [
        attr_str("http.method", method),
        attr_str("http.url", public_url),
        attr_int("http.status_code", status or 0),
        attr_str("server.address", host),
        attr_str("peer.service", peer),
        attr_str("page.path", page or random.choice(site.pages)),
    ]
    if extra_attrs:
        attrs.extend(extra_attrs)
    rum_spans.append(
        make_span(
            name=f"{method} {public_url}",
            kind=3,
            trace_id=trace_id,
            span_id=span_id,
            parent_span_id=parent_id,
            start_ns=start_ns,
            end_ns=end_ns,
            attributes=attrs,
            status_code=1 if ok else 2,
            status_message="" if ok else f"HTTP {status}",
        )
    )


# --- shop scenarios ---


def scenario_shop_browse(site: Site) -> None:
    trace_id = new_id(32)
    root_id = new_id(16)
    page = random.choice(site.pages)
    t0 = now_ns()
    t1 = t0 + int(random.uniform(80, 350) * 1e6)
    rum_spans = [
        make_span(
            name="documentLoad",
            kind=1,
            trace_id=trace_id,
            span_id=root_id,
            parent_span_id="",
            start_ns=t0,
            end_ns=t1,
            attributes=[
                attr_str("http.url", f"{site.origin}{page}"),
                attr_str("page.path", page),
                attr_str("navigation.type", "navigate"),
            ],
        )
    ]
    add_vitals(rum_spans, trace_id, root_id, page, t1)

    fetch_span_id = new_id(16)
    fetch_start = t1 + int(20 * 1e6)
    tp = f"00-{trace_id}-{fetch_span_id}-01"
    status, _ = get_json(
        f"{site.api_base}/api/products",
        headers={"Accept": "application/json", "traceparent": tp, "Origin": site.origin},
    )
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}/api/products",
        host=site.api_host,
        peer=site.api_peer,
        status=status,
        trace_id=trace_id,
        span_id=fetch_span_id,
        parent_id=root_id,
        start_ns=fetch_start,
        end_ns=now_ns(),
        page=page,
    )
    rum_spans.append(
        make_span(
            name=f"GET {site.origin}/css/style.css",
            kind=3,
            trace_id=trace_id,
            span_id=new_id(16),
            parent_span_id=root_id,
            start_ns=t0 + int(30 * 1e6),
            end_ns=t0 + int(random.uniform(40, 120) * 1e6),
            attributes=[
                attr_str("http.method", "GET"),
                attr_str("http.url", f"{site.origin}/css/style.css"),
                attr_int("http.status_code", 200),
                attr_str("resource.type", "stylesheet"),
                attr_str("page.path", page),
            ],
        )
    )
    export_rum(site, rum_spans, hint="" if 200 <= status < 400 else "error")
    print(f"[traffic][{site.service}] documentLoad {page} → {status}", flush=True)


def scenario_shop_spa(site: Site) -> None:
    trace_id = new_id(32)
    root_id = new_id(16)
    frm, to = random.sample(site.pages, 2)
    t0 = now_ns()
    t1 = t0 + int(random.uniform(30, 150) * 1e6)
    rum_spans = [
        make_span(
            name="routeChange",
            kind=1,
            trace_id=trace_id,
            span_id=root_id,
            parent_span_id="",
            start_ns=t0,
            end_ns=t1,
            attributes=[
                attr_str("page.path", to),
                attr_str("page.referrer", f"{site.origin}{frm}"),
                attr_str("navigation.type", "soft"),
                attr_str("http.url", f"{site.origin}{to}"),
            ],
        )
    ]
    add_vitals(rum_spans, trace_id, root_id, to, t1)
    fetch_span_id = new_id(16)
    tp = f"00-{trace_id}-{fetch_span_id}-01"
    status, _ = get_json(
        f"{site.api_base}/api/products",
        headers={"Accept": "application/json", "traceparent": tp, "Origin": site.origin},
    )
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}/api/products",
        host=site.api_host,
        peer=site.api_peer,
        status=status,
        trace_id=trace_id,
        span_id=fetch_span_id,
        parent_id=root_id,
        start_ns=t1,
        end_ns=now_ns(),
        page=to,
    )
    export_rum(site, rum_spans)
    print(f"[traffic][{site.service}] routeChange {frm}→{to} → {status}", flush=True)


def scenario_shop_checkout(site: Site, fail: bool) -> None:
    trace_id = new_id(32)
    root_id = new_id(16)
    root_name = "routeChange" if random.random() < 0.55 else "documentLoad"
    t0 = now_ns()
    t1 = t0 + int(120 * 1e6)
    rum_spans = [
        make_span(
            name=root_name,
            kind=1,
            trace_id=trace_id,
            span_id=root_id,
            parent_span_id="",
            start_ns=t0,
            end_ns=t1,
            attributes=[
                attr_str("http.url", f"{site.origin}/checkout.html"),
                attr_str("page.path", "/checkout.html"),
                attr_str("navigation.type", "soft" if root_name == "routeChange" else "navigate"),
            ],
        )
    ]
    add_vitals(rum_spans, trace_id, root_id, "/checkout.html", t1)
    fetch_span_id = new_id(16)
    fetch_start = now_ns()
    tp = f"00-{trace_id}-{fetch_span_id}-01"
    payload = {"email": "bot@example.com", "total": random.choice([49, 99, 218]), "fail": fail}
    code, _ = post_json(
        f"{site.checkout_base}/api/checkout",
        payload,
        headers={
            "Content-Type": "application/json",
            "Accept": "application/json",
            "traceparent": tp,
            "Origin": site.origin,
        },
    )
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="POST",
        public_url=f"{site.checkout_public}/api/checkout",
        host=site.checkout_host,
        peer=site.checkout_peer,
        status=code if not fail else (code or 402),
        trace_id=trace_id,
        span_id=fetch_span_id,
        parent_id=root_id,
        start_ns=fetch_start,
        end_ns=now_ns(),
        page="/checkout.html",
        extra_attrs=[attr_str("exception.message", "payment_declined" if fail else "")],
    )
    export_rum(site, rum_spans, hint="" if not fail and 200 <= code < 400 else "error")
    print(f"[traffic][{site.service}] checkout {'FAIL' if fail else 'ok'} → {code}", flush=True)


def scenario_shop_slow(site: Site) -> None:
    delay_ms = random.randint(2000, 8000)
    trace_id = new_id(32)
    root_id = new_id(16)
    page = random.choice(["/", "/shop.html"])
    t0 = now_ns()
    rum_spans = [
        make_span(
            name="documentLoad" if random.random() < 0.4 else "routeChange",
            kind=1,
            trace_id=trace_id,
            span_id=root_id,
            parent_span_id="",
            start_ns=t0,
            end_ns=t0 + int(80 * 1e6),
            attributes=[
                attr_str("http.url", f"{site.origin}{page}"),
                attr_str("page.path", page),
                attr_str("navigation.type", "navigate"),
            ],
        )
    ]
    fetch_span_id = new_id(16)
    fetch_start = now_ns()
    tp = f"00-{trace_id}-{fetch_span_id}-01"
    status, _ = get_json(
        f"{site.api_base}/api/products?delay_ms={delay_ms}",
        headers={
            "Accept": "application/json",
            "traceparent": tp,
            "Origin": site.origin,
            "X-Demo-Delay": str(delay_ms),
        },
        timeout=20,
    )
    end = now_ns()
    add_vitals(rum_spans, trace_id, root_id, page, end, slow=True, backend_delay_ms=delay_ms)
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}/api/products",
        host=site.api_host,
        peer=site.api_peer,
        status=status or 0,
        trace_id=trace_id,
        span_id=fetch_span_id,
        parent_id=root_id,
        start_ns=fetch_start,
        end_ns=end,
        page=page,
        extra_attrs=[attr_str("ui.action", "slow_catalog"), attr_int("demo.delay_ms", delay_ms)],
    )
    export_rum(site, rum_spans, hint="" if 200 <= (status or 0) < 400 else "error")
    print(f"[traffic][{site.service}] slow products delay={delay_ms}ms → {status}", flush=True)


def scenario_shop_5xx(site: Site) -> None:
    code = random.choice([500, 503, 504])
    trace_id = new_id(32)
    span_id = new_id(16)
    t0 = now_ns()
    tp = f"00-{trace_id}-{span_id}-01"
    status, _ = get_json(
        f"{site.api_base}/api/products?force_error={code}",
        headers={
            "Accept": "application/json",
            "traceparent": tp,
            "Origin": site.origin,
            "X-Demo-Error": str(code),
        },
    )
    rum_spans: list[dict[str, Any]] = []
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}/api/products",
        host=site.api_host,
        peer=site.api_peer,
        status=status or code,
        trace_id=trace_id,
        span_id=span_id,
        parent_id="",
        start_ns=t0,
        end_ns=now_ns(),
        extra_attrs=[attr_str("ui.action", "force_5xx"), attr_int("demo.force_error", code)],
    )
    export_rum(site, rum_spans, hint="error")
    print(f"[traffic][{site.service}] api 5xx force={code} → {status}", flush=True)


def scenario_shop_js_error(site: Site) -> None:
    trace_id = new_id(32)
    page = random.choice(site.pages)
    t0 = now_ns()
    rum_spans = [
        make_span(
            name=random.choice(["window.onerror", "unhandledrejection"]),
            kind=1,
            trace_id=trace_id,
            span_id=new_id(16),
            parent_span_id="",
            start_ns=t0,
            end_ns=t0 + int(2 * 1e6),
            attributes=[
                attr_str("exception.type", random.choice(["Error", "TypeError", "NetworkError"])),
                attr_str(
                    "exception.message",
                    random.choice(
                        [
                            "intentional demo error from traffic generator",
                            "Cannot read properties of undefined (reading 'price')",
                            "Failed to fetch",
                            "Script error.",
                        ]
                    ),
                ),
                attr_str("page.path", page),
                attr_str("http.url", f"{site.origin}{page}"),
            ],
            status_code=2,
            status_message="js_error",
        )
    ]
    export_rum(site, rum_spans, hint="error")
    print(f"[traffic][{site.service}] js error", flush=True)


def scenario_shop_xhr(site: Site) -> None:
    trace_id = new_id(32)
    span_id = new_id(16)
    t0 = now_ns()
    tp = f"00-{trace_id}-{span_id}-01"
    status, _ = get_json(
        f"{site.api_base}/api/products",
        headers={"Accept": "application/json", "traceparent": tp, "Origin": site.origin},
    )
    rum_spans: list[dict[str, Any]] = []
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}/api/products",
        host=site.api_host,
        peer=site.api_peer,
        status=status,
        trace_id=trace_id,
        span_id=span_id,
        parent_id="",
        start_ns=t0,
        end_ns=now_ns(),
        extra_attrs=[attr_str("ui.action", "refresh_catalog")],
    )
    export_rum(site, rum_spans, hint="" if 200 <= status < 400 else "error")
    print(f"[traffic][{site.service}] xhr-only → {status}", flush=True)


# --- portal scenarios ---


def scenario_portal_browse(site: Site) -> None:
    trace_id = new_id(32)
    root_id = new_id(16)
    page = random.choice(site.pages)
    t0 = now_ns()
    t1 = t0 + int(random.uniform(90, 400) * 1e6)
    rum_spans = [
        make_span(
            name="documentLoad",
            kind=1,
            trace_id=trace_id,
            span_id=root_id,
            parent_span_id="",
            start_ns=t0,
            end_ns=t1,
            attributes=[
                attr_str("http.url", f"{site.origin}{page}"),
                attr_str("page.path", page),
                attr_str("navigation.type", "navigate"),
            ],
        )
    ]
    add_vitals(rum_spans, trace_id, root_id, page, t1)
    fetch_span_id = new_id(16)
    fetch_start = t1 + int(20 * 1e6)
    tp = f"00-{trace_id}-{fetch_span_id}-01"
    path = "/api/tickets" if page != "/status.html" else "/api/status"
    status, _ = get_json(
        f"{site.api_base}{path}",
        headers={"Accept": "application/json", "traceparent": tp, "Origin": site.origin},
    )
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}{path}",
        host=site.api_host,
        peer=site.api_peer,
        status=status,
        trace_id=trace_id,
        span_id=fetch_span_id,
        parent_id=root_id,
        start_ns=fetch_start,
        end_ns=now_ns(),
        page=page,
    )
    export_rum(site, rum_spans, hint="" if 200 <= status < 400 else "error")
    print(f"[traffic][{site.service}] documentLoad {page} {path} → {status}", flush=True)


def scenario_portal_spa(site: Site) -> None:
    trace_id = new_id(32)
    root_id = new_id(16)
    frm, to = random.sample(site.pages, 2)
    t0 = now_ns()
    t1 = t0 + int(random.uniform(40, 180) * 1e6)
    rum_spans = [
        make_span(
            name="routeChange",
            kind=1,
            trace_id=trace_id,
            span_id=root_id,
            parent_span_id="",
            start_ns=t0,
            end_ns=t1,
            attributes=[
                attr_str("page.path", to),
                attr_str("page.referrer", f"{site.origin}{frm}"),
                attr_str("navigation.type", "soft"),
                attr_str("http.url", f"{site.origin}{to}"),
            ],
        )
    ]
    add_vitals(rum_spans, trace_id, root_id, to, t1)
    fetch_span_id = new_id(16)
    tp = f"00-{trace_id}-{fetch_span_id}-01"
    path = "/api/status" if to == "/status.html" else "/api/tickets"
    status, _ = get_json(
        f"{site.api_base}{path}",
        headers={"Accept": "application/json", "traceparent": tp, "Origin": site.origin},
    )
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}{path}",
        host=site.api_host,
        peer=site.api_peer,
        status=status,
        trace_id=trace_id,
        span_id=fetch_span_id,
        parent_id=root_id,
        start_ns=t1,
        end_ns=now_ns(),
        page=to,
    )
    export_rum(site, rum_spans)
    print(f"[traffic][{site.service}] routeChange {frm}→{to} → {status}", flush=True)


def scenario_portal_create_ticket(site: Site, fail: bool = False) -> None:
    trace_id = new_id(32)
    root_id = new_id(16)
    t0 = now_ns()
    t1 = t0 + int(100 * 1e6)
    rum_spans = [
        make_span(
            name="routeChange",
            kind=1,
            trace_id=trace_id,
            span_id=root_id,
            parent_span_id="",
            start_ns=t0,
            end_ns=t1,
            attributes=[
                attr_str("http.url", f"{site.origin}/tickets.html"),
                attr_str("page.path", "/tickets.html"),
                attr_str("navigation.type", "soft"),
            ],
        )
    ]
    add_vitals(rum_spans, trace_id, root_id, "/tickets.html", t1, slow=fail)
    fetch_span_id = new_id(16)
    fetch_start = now_ns()
    tp = f"00-{trace_id}-{fetch_span_id}-01"
    payload = {
        "subject": random.choice(["Cannot checkout", "Wrong size", "Refund request", "Account locked"]),
        "email": "user@example.com",
        "fail": fail,
    }
    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json",
        "traceparent": tp,
        "Origin": site.origin,
    }
    if fail:
        headers["X-Demo-Fail"] = "1"
    code, _ = post_json(f"{site.api_base}/api/tickets", payload, headers=headers)
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="POST",
        public_url=f"{site.api_public}/api/tickets",
        host=site.api_host,
        peer=site.api_peer,
        status=code if not fail else (code or 500),
        trace_id=trace_id,
        span_id=fetch_span_id,
        parent_id=root_id,
        start_ns=fetch_start,
        end_ns=now_ns(),
        page="/tickets.html",
        extra_attrs=[attr_str("ui.action", "create_ticket")],
    )
    export_rum(site, rum_spans, hint="" if not fail and 200 <= code < 400 else "error")
    print(f"[traffic][{site.service}] create ticket fail={fail} → {code}", flush=True)


def scenario_portal_slow(site: Site) -> None:
    delay_ms = random.randint(2000, 7000)
    trace_id = new_id(32)
    root_id = new_id(16)
    page = "/tickets.html"
    t0 = now_ns()
    rum_spans = [
        make_span(
            name="documentLoad",
            kind=1,
            trace_id=trace_id,
            span_id=root_id,
            parent_span_id="",
            start_ns=t0,
            end_ns=t0 + int(90 * 1e6),
            attributes=[
                attr_str("http.url", f"{site.origin}{page}"),
                attr_str("page.path", page),
                attr_str("navigation.type", "navigate"),
            ],
        )
    ]
    fetch_span_id = new_id(16)
    fetch_start = now_ns()
    tp = f"00-{trace_id}-{fetch_span_id}-01"
    status, _ = get_json(
        f"{site.api_base}/api/tickets?delay_ms={delay_ms}",
        headers={
            "Accept": "application/json",
            "traceparent": tp,
            "Origin": site.origin,
            "X-Demo-Delay": str(delay_ms),
        },
        timeout=20,
    )
    end = now_ns()
    add_vitals(rum_spans, trace_id, root_id, page, end, slow=True, backend_delay_ms=delay_ms)
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}/api/tickets",
        host=site.api_host,
        peer=site.api_peer,
        status=status or 0,
        trace_id=trace_id,
        span_id=fetch_span_id,
        parent_id=root_id,
        start_ns=fetch_start,
        end_ns=end,
        page=page,
        extra_attrs=[attr_int("demo.delay_ms", delay_ms)],
    )
    export_rum(site, rum_spans)
    print(f"[traffic][{site.service}] slow tickets delay={delay_ms}ms → {status}", flush=True)


def scenario_portal_5xx(site: Site) -> None:
    code = random.choice([500, 503, 504])
    trace_id = new_id(32)
    span_id = new_id(16)
    t0 = now_ns()
    tp = f"00-{trace_id}-{span_id}-01"
    status, _ = get_json(
        f"{site.api_base}/api/tickets?force_error={code}",
        headers={
            "Accept": "application/json",
            "traceparent": tp,
            "Origin": site.origin,
            "X-Demo-Error": str(code),
        },
    )
    rum_spans: list[dict[str, Any]] = []
    http_client_span(
        site=site,
        rum_spans=rum_spans,
        method="GET",
        public_url=f"{site.api_public}/api/tickets",
        host=site.api_host,
        peer=site.api_peer,
        status=status or code,
        trace_id=trace_id,
        span_id=span_id,
        parent_id="",
        start_ns=t0,
        end_ns=now_ns(),
        extra_attrs=[attr_int("demo.force_error", code)],
    )
    export_rum(site, rum_spans, hint="error")
    print(f"[traffic][{site.service}] portal 5xx force={code} → {status}", flush=True)


def scenario_portal_js_error(site: Site) -> None:
    trace_id = new_id(32)
    page = random.choice(site.pages)
    t0 = now_ns()
    rum_spans = [
        make_span(
            name="window.onerror",
            kind=1,
            trace_id=trace_id,
            span_id=new_id(16),
            parent_span_id="",
            start_ns=t0,
            end_ns=t0 + int(2 * 1e6),
            attributes=[
                attr_str("exception.type", "Error"),
                attr_str("exception.message", "portal widget failed to mount"),
                attr_str("page.path", page),
                attr_str("http.url", f"{site.origin}{page}"),
            ],
            status_code=2,
            status_message="js_error",
        )
    ]
    export_rum(site, rum_spans, hint="error")
    print(f"[traffic][{site.service}] js error", flush=True)


def pick_and_run() -> None:
    site = pick_site()
    r = random.random()
    if site.kind == "portal":
        if r < SLOW_RATE * 0.7:
            scenario_portal_slow(site)
        elif r < SLOW_RATE:
            scenario_portal_5xx(site)
        else:
            r2 = (r - SLOW_RATE) / max(1e-9, 1.0 - SLOW_RATE)
            if r2 < 0.30:
                scenario_portal_browse(site)
            elif r2 < 0.52:
                scenario_portal_spa(site)
            elif r2 < 0.72:
                scenario_portal_create_ticket(site, fail=False)
            elif r2 < 0.72 + ERROR_RATE * 0.35:
                scenario_portal_create_ticket(site, fail=True)
            elif r2 < 0.72 + ERROR_RATE * 0.60:
                scenario_portal_5xx(site)
            else:
                scenario_portal_js_error(site)
        return

    # shop
    if r < SLOW_RATE * 0.55:
        scenario_shop_slow(site)
    elif r < SLOW_RATE * 0.80:
        # reuse slow products weight; occasional checkout slow via create path
        scenario_shop_slow(site)
    elif r < SLOW_RATE:
        scenario_shop_5xx(site)
    else:
        r2 = (r - SLOW_RATE) / max(1e-9, 1.0 - SLOW_RATE)
        if r2 < 0.24:
            scenario_shop_browse(site)
        elif r2 < 0.42:
            scenario_shop_spa(site)
        elif r2 < 0.55:
            scenario_shop_xhr(site)
        elif r2 < 0.72:
            scenario_shop_checkout(site, fail=False)
        elif r2 < 0.72 + ERROR_RATE * 0.35:
            scenario_shop_checkout(site, fail=True)
        elif r2 < 0.72 + ERROR_RATE * 0.55:
            scenario_shop_5xx(site)
        else:
            scenario_shop_js_error(site)


def main() -> None:
    desc = ", ".join(f"{s.service}@{s.origin}→{s.api_base}" for s in SITES)
    print(
        f"[traffic] started interval={INTERVAL_SEC}s±{JITTER_SEC}s error_rate={ERROR_RATE} "
        f"slow_rate={SLOW_RATE} sites=[{desc}] otlp={COROOT_OTLP}",
        flush=True,
    )
    time.sleep(5)
    while True:
        try:
            pick_and_run()
        except Exception as e:
            print(f"[traffic] scenario error: {e}", flush=True)
        delay = max(2.0, INTERVAL_SEC + random.uniform(-JITTER_SEC, JITTER_SEC))
        time.sleep(delay)


if __name__ == "__main__":
    main()
