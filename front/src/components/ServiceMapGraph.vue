<template>
    <div class="graph-wrapper" v-on-resize="scheduleResize" @mouseleave="leave">
        <div
            ref="graph"
            class="graph"
            :class="{ hidden: layingOut }"
            role="img"
            :aria-label="`Service map topology: ${counts.nodes} applications, ${counts.links} connections. Switch to the Tiers view for a list of applications and links.`"
            @pointermove="pointerMove"
            @pointerdown.capture="pointerMove"
            @wheel.capture.passive="fitPending = false"
            @mousedown.middle="auxDown"
            @auxclick="auxClick"
            @contextmenu="contextMenu"
        />
        <div v-if="layingOut" class="laying-out grey--text">Laying out…</div>

        <div class="toolbar">
            <v-tooltip v-for="t in tools" :key="t.label" left>
                <template #activator="{ on, attrs }">
                    <v-btn
                        icon
                        small
                        v-bind="attrs"
                        v-on="on"
                        :color="t.pressed ? 'primary' : undefined"
                        :aria-label="t.label"
                        :aria-pressed="t.pressed === undefined ? undefined : String(t.pressed)"
                        @click="t.action"
                    >
                        <v-icon small>{{ t.icon }}</v-icon>
                    </v-btn>
                </template>
                <v-card class="px-2 py-1">{{ t.label }}</v-card>
            </v-tooltip>
        </div>

        <div
            v-show="menuApp"
            ref="menuBtn"
            class="menu-btn"
            @mouseenter="menuBtnHovered = true"
            @mouseleave="
                menuBtnHovered = false;
                scheduleMenuBtnClose();
            "
        >
            <AppPreferences v-if="menuApp" :key="menuApp.id" :app="menuApp" :categories="categories" />
        </div>

        <v-card v-if="tooltip" ref="tooltip" class="tooltip pa-2">
            <template v-if="tooltip.kind === 'node'">
                <div class="font-weight-medium">{{ tooltip.name }}</div>
                <div class="grey--text text-caption">
                    <span v-if="tooltip.ns">ns: {{ tooltip.ns }}</span>
                    <span v-if="tooltip.cluster"> · cluster: {{ tooltip.cluster }}</span>
                </div>
                <div v-for="i in tooltip.indicators">
                    <Led :status="i.status" />
                    <span>{{ i.message }}</span>
                </div>
            </template>
            <template v-else>
                <div class="grey--text text-caption">{{ tooltip.from }} → {{ tooltip.to }}</div>
                <div v-for="s in tooltip.stats">{{ s }}</div>
                <div v-if="!tooltip.stats.length" class="grey--text">no stats</div>
            </template>
        </v-card>
    </div>
</template>

<script>
import ForceGraph from 'force-graph';
import { forceCollide, forceManyBody, forceX, forceY } from 'd3-force-3d';
import AppPreferences from '@/components/AppPreferences.vue';
import Led from '@/components/Led.vue';

const statuses = ['unknown', 'ok', 'warning', 'critical'];
const forces = {
    charge: -220,
    chargeDistanceMax: 800,
    linkStrength: 0.4,
    collidePadding: 12,
    centerX: 0.04,
    centerY: 0.06,
    flowStrength: 0.15,
    flowLevelDistance: 160,
};
const engine = {
    alphaMin: 0.001,
    alphaDecay: 0.03,
    velocityDecay: 0.35,
    cooldownTime: 10000,
    earlyFitTicks: 40,
    settleTicks: 80,
};
const durations = { zoom: 700, fit: 1000, resize: 150, menuClose: 400 };
const sizes = { icon: 256, minSprite: 64, maxSprite: 512, labelFont: 10, statsFont: 11, fitPadding: 40, fitLabel: 30, maxLabel: 28, maxSubLabel: 36 };
const zoomLimits = { max: 4, fit: 2 };
const max = (values) => values.reduce((m, v) => (v > m ? v : m), 0);
const particleColor = '#0b8a3e';

export default {
    props: {
        applications: Array,
        categories: Array,
    },

    components: { AppPreferences, Led },

    data() {
        return {
            tooltip: null, // frozen plain objects only: Vue must not observe force-graph's simulation objects
            menuApp: null,
            counts: { nodes: 0, links: 0 },
            layingOut: false, // with reduced motion, the map is hidden while the layout settles instead of animating
            flow: this.$storage.local('service-map-graph-flow') !== false,
        };
    },

    created() {
        // non-reactive: the graph data is mutated by the simulation
        this.fg = null;
        this.nodes = new Map();
        this.links = [];
        this.adj = new Map();
        this.maxLevel = 0;
        this.hover = null;
        this.hoverLabel = null;
        this.pinned = null;
        this.pointer = null;
        this.graphPointer = null;
        this.menuNode = null;
        this.menuBtnHovered = false;
        this.menuBtnTimer = null;
        this.images = {};
        this.colors = {};
        this.alphaCache = new Map();
        this.sprites = new Map();
        this.viewport = null;
        this.ctx2d = document.createElement('canvas').getContext('2d');
        this.fitPending = true;
        this.ticks = 0;
        this.settlePinned = [];
        this.hoverRefreshPending = false;
        this.updatePending = false;
        this.resizeTimer = null;
        this.parentObserver = null;
        this.motionQuery = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;
        this.reducedMotion = !!(this.motionQuery && this.motionQuery.matches);
    },

    mounted() {
        this.readColors();
        this.init();
        this.update();
        document.fonts?.ready.then(() => this.measureLabels());
    },

    beforeDestroy() {
        window.removeEventListener('resize', this.scheduleResize);
        this.parentObserver && this.parentObserver.disconnect();
        clearTimeout(this.resizeTimer);
        clearTimeout(this.menuBtnTimer);
        if (this.motionQuery && this.motionQuery.removeEventListener) {
            this.motionQuery.removeEventListener('change', this.motionChanged);
        }
        if (this.fg) {
            this.fg._destructor();
            this.fg = null;
        }
    },

    watch: {
        applications() {
            // a refresh changes both the applications and the filter: a single update for both
            if (this.updatePending) {
                return;
            }
            this.updatePending = true;
            this.$nextTick(() => {
                this.updatePending = false;
                this.update();
            });
        },
        multicluster() {
            this.measureLabels();
        },
        flow(flow) {
            this.$storage.local('service-map-graph-flow', flow);
            this.applyXForce();
            this.releaseSettle();
            this.startLayout();
            this.fg.d3ReheatSimulation();
        },
        dark() {
            this.$nextTick(() => {
                this.readColors();
                this.redraw();
            });
        },
    },

    computed: {
        tools() {
            return [
                { label: 'Zoom in', icon: 'mdi-plus', action: () => this.zoomBy(1.5) },
                { label: 'Zoom out', icon: 'mdi-minus', action: () => this.zoomBy(1 / 1.5) },
                { label: 'Fit to screen', icon: 'mdi-fit-to-screen-outline', action: this.fit },
                { label: 'Re-layout', icon: 'mdi-refresh', action: this.relayout },
                { label: 'Flow left to right', icon: 'mdi-arrow-right-bold-box-outline', action: () => (this.flow = !this.flow), pressed: this.flow },
            ];
        },
        multicluster() {
            return this.$api.context.multicluster;
        },
        dark() {
            return this.$vuetify.theme.dark;
        },
    },

    methods: {
        init() {
            const fg = ForceGraph()(this.$refs.graph)
                .backgroundColor('rgba(0,0,0,0)')
                .nodeId('id')
                .nodeRelSize(1)
                .nodeVal((n) => n.r * n.r)
                .nodeLabel(() => '')
                .linkLabel(() => '')
                .showPointerCursor(false)
                .maxZoom(zoomLimits.max)
                .nodeCanvasObject(this.drawNode)
                .nodePointerAreaPaint((n, color, ctx) => {
                    ctx.fillStyle = color;
                    ctx.beginPath();
                    ctx.arc(n.x, n.y, n.r, 0, 2 * Math.PI);
                    ctx.fill();
                    if (n.labelShown) {
                        const b = n.labelBox;
                        ctx.fillRect(b.x0, b.y0, b.x1 - b.x0, b.y1 - b.y0);
                    }
                })
                .linkColor(this.linkColor)
                .linkWidth((l) => (this.hover && this.isHiLink(l) ? 1 + 3 * (l.hr || 0) : 1 + l.w * 2.5))
                .linkDirectionalArrowLength((l) => (this.isHiLink(l) ? 5 : 0))
                .linkDirectionalArrowRelPos(1)
                .linkLineDash((l) => (l.status === 'unknown' ? [4, 4] : l.status === 'warning' || l.status === 'critical' ? [6, 4] : null))
                .linkDirectionalParticles((l) =>
                    !this.reducedMotion && this.hover && this.isHiLink(l) && l.weight > 0 ? Math.max(1, Math.round((l.hr || 0) * 8)) : 0,
                )
                .linkDirectionalParticleSpeed(0.006)
                .linkDirectionalParticleWidth((l) => 2 + 2.5 * (l.hr || 0))
                .linkDirectionalParticleColor((l) => (l.status === 'critical' ? this.colors.critical : particleColor))
                .onNodeHover(this.nodeHover)
                .onLinkHover(this.linkHover)
                .onNodeClick(this.nodeClick)
                .onNodeDrag(() => {
                    this.tooltip = null;
                    // force-graph re-heats a dragged layout via alphaTarget, but alpha only rises on the next tick,
                    // and d3AlphaMin would stop a settled engine before that tick
                    this.fg.d3AlphaMin(0);
                })
                .onNodeDragEnd((n) => {
                    this.fg.d3AlphaMin(engine.alphaMin);
                    if (!this.settlePinned.includes(n)) {
                        n.fx = undefined;
                        n.fy = undefined;
                    }
                })
                .onBackgroundClick(() => {
                    if (this.pinned) {
                        this.pinned = null;
                        this.nodeHover(null);
                    }
                })
                .onZoom(() => (this.tooltip = null))
                .onRenderFramePre(() => {
                    const a = this.fg.screen2GraphCoords(0, 0);
                    const b = this.fg.screen2GraphCoords(this.fg.width(), this.fg.height());
                    this.viewport = { x0: a.x, y0: a.y, x1: b.x, y1: b.y };
                })
                .onRenderFramePost((ctx, scale) => {
                    // after the frame: force-graph paints particles and arrows after the links
                    if (this.hover) {
                        this.links.forEach((l) => l.stats.length && this.isHiLink(l) && this.drawLinkStats(l, ctx, scale));
                    }
                    this.placeMenuBtn();
                })
                .onEngineTick(this.engineTick)
                .onEngineStop(this.engineStop)
                .cooldownTime(engine.cooldownTime)
                .d3AlphaMin(engine.alphaMin)
                .d3AlphaDecay(engine.alphaDecay)
                .d3VelocityDecay(engine.velocityDecay);

            fg.d3Force('charge', forceManyBody().strength(forces.charge).distanceMax(forces.chargeDistanceMax));
            fg.d3Force('link')
                .distance((l) => Math.min(50 + Math.max(l.source.deg, l.target.deg) * 3, 180))
                .strength(forces.linkStrength);
            fg.d3Force('collide', forceCollide((n) => n.r + forces.collidePadding).iterations(2));
            fg.d3Force('y', forceY(0).strength(forces.centerY));
            this.fg = fg;
            this.resize();
            // the height also depends on the window and on the content above the map
            window.addEventListener('resize', this.scheduleResize);
            if (window.ResizeObserver && this.$el.parentElement) {
                this.parentObserver = new ResizeObserver(this.scheduleResize);
                this.parentObserver.observe(this.$el.parentElement);
            }
            if (this.motionQuery && this.motionQuery.addEventListener) {
                this.motionQuery.addEventListener('change', this.motionChanged);
            }
        },

        update() {
            if (!this.fg) {
                return;
            }
            const apps = this.applications || [];
            const prev = this.nodes;
            const nodes = new Map();
            apps.forEach((a) => {
                const id = this.$utils.appId(a.id);
                const n = prev.get(a.id) || { id: a.id, labelBox: { x0: 0, x1: 0, y0: 0, y1: 0 } };
                n.app = a;
                n.name = id.name;
                n.kind = id.kind;
                n.status = a.status || 'unknown';
                n.icon = a.icon;
                n.deg = 0;
                nodes.set(a.id, n);
                this.loadImage(a.icon);
            });
            const links = [];
            const seen = new Set();
            apps.forEach((a) => {
                (a.upstreams || []).forEach((u) => {
                    const id = a.id + '->' + u.id;
                    if (u.id === a.id || !nodes.has(u.id) || seen.has(id)) {
                        return;
                    }
                    seen.add(id);
                    const weight = u.weight > 0 ? u.weight : 0;
                    links.push({ id, source: a.id, target: u.id, status: u.status || 'unknown', weight, stats: u.stats || [] });
                    nodes.get(a.id).deg++;
                    nodes.get(u.id).deg++;
                });
            });
            const maxW = Math.max(1, max(links.map((l) => l.weight)));
            links.forEach((l) => {
                l.w = Math.log1p(l.weight) / Math.log1p(maxW);
                l.bidirectional = seen.has(l.target + '->' + l.source);
            });
            nodes.forEach((n) => (n.r = 12 + Math.sqrt(n.deg) * 2));

            const adj = new Map();
            nodes.forEach((n, id) => adj.set(id, new Set([id])));
            links.forEach((l) => {
                adj.get(l.source).add(l.target);
                adj.get(l.target).add(l.source);
            });

            const byId = new Map(links.map((l) => [l.id, l]));
            const sameNodes = nodes.size === prev.size && [...nodes.keys()].every((id) => prev.has(id));
            const sameLinks = links.length === this.links.length && this.links.every((l) => byId.has(l.id));
            this.adj = adj;
            this.nodes = nodes;
            this.counts = { nodes: nodes.size, links: links.length };
            this.calcLevels(nodes, links);
            this.applyXForce();

            if (sameNodes && sameLinks) {
                this.links.forEach((l) => {
                    const u = byId.get(l.id);
                    Object.assign(l, { status: u.status, weight: u.weight, stats: u.stats, w: u.w });
                });
                this.measureLabels();
                this.refreshHover();
                this.redraw();
                return;
            }

            const added = [...nodes.keys()].filter((id) => !prev.has(id));
            if (!prev.size || added.length > nodes.size * 0.2) {
                this.startLayout();
            } else {
                // a few apps came or went: new ones start next to their neighbours, the others stay while they settle
                added.forEach((id) => {
                    const n = nodes.get(id);
                    const placed = [...adj.get(id)].map((i) => nodes.get(i)).filter((m) => m !== n && m.x !== undefined);
                    if (placed.length) {
                        n.x = placed.reduce((s, m) => s + m.x, 0) / placed.length + (Math.random() - 0.5) * 40;
                        n.y = placed.reduce((s, m) => s + m.y, 0) / placed.length + (Math.random() - 0.5) * 40;
                    }
                });
                this.releaseSettle();
                nodes.forEach((n) => {
                    if (prev.has(n.id) && n.x !== undefined && n.fx === undefined) {
                        n.fx = n.x;
                        n.fy = n.y;
                        this.settlePinned.push(n);
                    }
                });
            }
            this.links = links;
            this.measureLabels();
            this.ticks = 0;
            // after force-graph has applied the data: it resolves link ends asynchronously
            this.hoverRefreshPending = true;
            this.fg.graphData({ nodes: [...nodes.values()], links });
        },

        releaseSettle() {
            this.settlePinned.forEach((n) => {
                n.fx = undefined;
                n.fy = undefined;
            });
            this.settlePinned = [];
        },

        engineTick() {
            this.ticks++;
            if (this.hoverRefreshPending && this.dataApplied()) {
                this.hoverRefreshPending = false;
                this.refreshHover();
            }
            if (this.settlePinned.length && this.ticks >= engine.settleTicks) {
                this.releaseSettle();
            }
            if (this.fitPending && this.ticks === engine.earlyFitTicks) {
                this.fitView(durations.fit);
            }
        },

        startLayout() {
            this.fitPending = true;
            this.ticks = 0;
            this.layingOut = this.reducedMotion;
        },

        engineStop() {
            this.layingOut = false;
            if (this.hoverRefreshPending && this.dataApplied()) {
                this.hoverRefreshPending = false;
                this.refreshHover();
            }
            this.releaseSettle();
            if (this.fitPending) {
                this.fit();
            }
        },

        readColors() {
            const s = getComputedStyle(this.$el);
            const v = (name, def) => s.getPropertyValue(name).trim() || def;
            const c = {
                text: v('--text-color', '#212121'),
                textDimmed: v('--text-color-dimmed', '#757575'),
                bg: v('--background-color', '#fff'),
                border: v('--border-color', '#d0d0d0'),
                selected: this.$vuetify.theme.currentTheme.primary || '#1976d2',
            };
            statuses.forEach((st) => (c[st] = v('--status-' + st, 'grey')));
            this.colors = c;
            this.sprites = new Map();
        },

        withAlpha(color, alpha) {
            const key = color + '|' + alpha;
            let res = this.alphaCache.get(key);
            if (res === undefined) {
                this.ctx2d.fillStyle = color;
                const c = this.ctx2d.fillStyle;
                const m = c.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)$/);
                if (c.startsWith('#')) {
                    const [r, g, b] = [1, 3, 5].map((i) => parseInt(c.slice(i, i + 2), 16));
                    res = `rgba(${r}, ${g}, ${b}, ${alpha})`;
                } else if (m) {
                    res = `rgba(${m[1]}, ${m[2]}, ${m[3]}, ${(m[4] === undefined ? 1 : +m[4]) * alpha})`;
                } else {
                    res = color;
                }
                this.alphaCache.set(key, res);
            }
            return res;
        },

        loadImage(icon) {
            if (!icon || icon in this.images) {
                return;
            }
            this.images[icon] = null;
            fetch(`${this.$coroot.base_path}static/img/tech-icons/${icon}.svg`)
                .then((r) => (r.ok ? r.text() : Promise.reject(r.status)))
                .then((svg) => {
                    // the icons have only a viewBox: Firefox can't draw SVGs without an intrinsic size on a canvas,
                    // and drawing an SVG on every frame is slow, so it's rasterized once
                    const size = sizes.icon;
                    const sized = svg.replace(/<svg\b([^>]*)>/, (m, attrs) =>
                        /\swidth=/.test(attrs) ? m : `<svg width="${size}" height="${size}"${attrs}>`,
                    );
                    const img = new Image();
                    img.onload = () => {
                        const canvas = document.createElement('canvas');
                        canvas.width = canvas.height = size;
                        const f = size / Math.max(img.naturalWidth || size, img.naturalHeight || size);
                        const [w, h] = [(img.naturalWidth || size) * f, (img.naturalHeight || size) * f];
                        canvas.getContext('2d').drawImage(img, (size - w) / 2, (size - h) / 2, w, h);
                        this.images[icon] = canvas;
                        this.redraw();
                    };
                    img.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(sized);
                })
                .catch(() => {});
        },

        measureLabels() {
            const ctx = this.ctx2d;
            const truncate = (s, max) => (s.length > max ? s.slice(0, max - 1) + '…' : s);
            this.nodes.forEach((n) => {
                const ns = n.app.labels && n.app.labels.ns;
                const sub = [this.multicluster && n.app.cluster, ns && 'ns:' + ns].filter(Boolean).join(' / ');
                n.label = truncate(n.name, sizes.maxLabel);
                n.sub = truncate(sub, sizes.maxSubLabel);
                ctx.font = `${sizes.labelFont}px Roboto, sans-serif`;
                n.labelW = ctx.measureText(n.label).width;
                ctx.font = `${sizes.labelFont * 0.85}px Roboto, sans-serif`;
                n.subW = n.sub ? ctx.measureText(n.sub).width : 0;
            });
            ctx.font = `${sizes.statsFont}px Roboto, sans-serif`;
            this.links.forEach((l) => (l.statsW = max(l.stats.map((st) => ctx.measureText(st).width))));
            this.redraw();
        },

        isHiLink(l) {
            return !this.hover || l.source.id === this.hover || l.target.id === this.hover;
        },

        linkColor(l) {
            const c = this.colors;
            if (!this.isHiLink(l)) {
                return this.withAlpha(c.textDimmed, 0.06);
            }
            const color = c[l.status] || c.unknown;
            return this.hover ? color : this.withAlpha(color, l.status === 'ok' ? 0.45 : 0.7);
        },

        sprite(key, draw, screenSize) {
            const px = Math.min(sizes.maxSprite, Math.max(sizes.minSprite, 2 ** Math.ceil(Math.log2(screenSize * (window.devicePixelRatio || 1)))));
            key += '@' + px;
            let c = this.sprites.get(key);
            if (!c) {
                c = document.createElement('canvas');
                c.width = c.height = px;
                draw(c.getContext('2d'), px / 2);
                this.sprites.set(key, c);
            }
            return c;
        },

        drawNode(n, ctx, scale) {
            const c = this.colors;
            const v = this.viewport;
            const margin = n.r * 3 + 40 / scale;
            n.labelShown = false;
            if (v && (n.x < v.x0 - margin || n.x > v.x1 + margin || n.y < v.y0 - margin || n.y > v.y1 + margin)) {
                return;
            }
            const neighbours = this.hover && this.adj.get(this.hover);
            const hi = !neighbours || neighbours.has(n.id);
            ctx.globalAlpha = hi ? 1 : 0.12;

            ctx.beginPath();
            ctx.arc(n.x, n.y, n.r, 0, 2 * Math.PI);
            ctx.fillStyle = c.bg;
            ctx.fill();
            ctx.lineWidth = n.id === this.hover ? 2.5 : 1.2;
            ctx.strokeStyle = n.id === this.hover ? c.selected : c.border;
            ctx.stroke();

            const img = n.icon && this.images[n.icon];
            if (img) {
                const s = n.r * 1.1;
                ctx.drawImage(img, n.x - s / 2, n.y - s / 2, s, s);
            } else {
                const text = n.kind === 'ExternalService' ? 'EXT' : (n.kind || '?').slice(0, 3).toUpperCase();
                const kind = this.sprite(
                    'kind:' + text,
                    (sc, r) => {
                        sc.fillStyle = c.textDimmed;
                        sc.font = `600 ${r * 0.55}px Roboto, sans-serif`;
                        sc.textAlign = 'center';
                        sc.textBaseline = 'middle';
                        sc.fillText(text, r, r);
                    },
                    2 * n.r * scale,
                );
                ctx.drawImage(kind, n.x - n.r, n.y - n.r, 2 * n.r, 2 * n.r);
            }

            const br = (n.r * 0.36) / 0.9;
            const badge = this.sprite('badge:' + n.status, (sc, r) => this.drawBadge(sc, r, r, r * 0.9, n.status), 2 * br * scale);
            ctx.drawImage(badge, n.x + n.r * 0.72 - br, n.y - n.r * 0.72 - br, 2 * br, 2 * br);

            const m = this.menuBtnPos(n);
            if (m.r * scale > 5 && n.id !== this.menuNode) {
                const mr = m.r / 0.9;
                const btn = this.sprite(
                    'menu',
                    (sc, r) => {
                        sc.beginPath();
                        sc.arc(r, r, r * 0.9, 0, 2 * Math.PI);
                        sc.fillStyle = c.bg;
                        sc.fill();
                        sc.lineWidth = r * 0.05;
                        sc.strokeStyle = c.border;
                        sc.stroke();
                        sc.fillStyle = c.textDimmed;
                        [-0.4, 0, 0.4].forEach((dy) => {
                            sc.beginPath();
                            sc.arc(r, r + dy * r, r * 0.12, 0, 2 * Math.PI);
                            sc.fill();
                        });
                    },
                    2 * mr * scale,
                );
                ctx.drawImage(btn, m.x - mr, m.y - mr, 2 * mr, 2 * mr);
            }

            n.labelShown = scale > 0.7 || (!!this.hover && hi);
            if (n.labelShown) {
                const fs = Math.max(10 / scale, 4);
                const k = fs / sizes.labelFont;
                const y = n.y + n.r + 2;
                const onLabel = n.id === this.hoverLabel;
                const underline = Math.max(fs * 0.08, 0.5);
                ctx.textAlign = 'center';
                ctx.textBaseline = 'top';

                ctx.font = `${fs}px Roboto, sans-serif`;
                ctx.fillStyle = onLabel ? c.selected : c.text;
                ctx.fillText(n.label, n.x, y);
                const w = n.labelW * k;
                if (onLabel) {
                    ctx.fillRect(n.x - w / 2, y + fs * 1.1, w, underline);
                }
                let width = w;
                let bottom = y + fs * 1.2;

                if (n.sub) {
                    const sfs = fs * 0.85;
                    const sy = y + fs * 1.25;
                    const sw = n.subW * k;
                    ctx.font = `${sfs}px Roboto, sans-serif`;
                    ctx.fillStyle = onLabel ? c.selected : c.textDimmed;
                    ctx.fillText(n.sub, n.x, sy);
                    if (onLabel) {
                        ctx.fillRect(n.x - sw / 2, sy + sfs * 1.1, sw, underline);
                    }
                    width = Math.max(width, sw);
                    bottom = sy + sfs * 1.2;
                }
                const pad = fs * 0.2;
                const b = n.labelBox;
                b.x0 = n.x - width / 2 - pad;
                b.x1 = n.x + width / 2 + pad;
                b.y0 = y - pad;
                b.y1 = bottom + pad;
            }
            ctx.globalAlpha = 1;
        },

        drawLinkStats(l, ctx, scale) {
            const s = l.source;
            const t = l.target;
            if (s.x === undefined || t.x === undefined) {
                return;
            }
            const c = this.colors;
            // two-way traffic comes as two overlapping links: each box is shifted towards its own source
            const k = l.bidirectional ? 0.35 : 0.5;
            const x = s.x + (t.x - s.x) * k;
            const y = s.y + (t.y - s.y) * k;
            const fs = sizes.statsFont / scale;
            const lh = fs * 1.25;
            const pad = fs * 0.35;
            const w = (l.statsW || 0) / scale + pad * 2;
            const h = lh * l.stats.length + pad * 2 - (lh - fs);
            const x0 = x - w / 2;
            const y0 = y - h / 2;
            ctx.font = `${fs}px Roboto, sans-serif`;
            ctx.fillStyle = this.withAlpha(c.bg, 0.92);
            ctx.strokeStyle = l.status === 'critical' || l.status === 'warning' ? c[l.status] : c.border;
            ctx.lineWidth = 1 / scale;
            ctx.beginPath();
            if (ctx.roundRect) {
                ctx.roundRect(x0, y0, w, h, 3 / scale);
            } else {
                ctx.rect(x0, y0, w, h);
            }
            ctx.fill();
            ctx.stroke();
            ctx.fillStyle = c.text;
            ctx.textAlign = 'left';
            ctx.textBaseline = 'top';
            l.stats.forEach((st, i) => ctx.fillText(st, x0 + pad, y0 + pad + i * lh));
        },

        drawBadge(ctx, x, y, r, status) {
            const c = this.colors;
            ctx.beginPath();
            ctx.arc(x, y, r, 0, 2 * Math.PI);
            ctx.fillStyle = c[status] || c.unknown;
            ctx.fill();
            ctx.lineWidth = r * 0.18;
            ctx.strokeStyle = c.bg;
            ctx.stroke();
            ctx.lineWidth = r * 0.22;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            ctx.strokeStyle = status === 'warning' ? '#5a4a00' : '#fff';
            ctx.beginPath();
            if (status === 'ok') {
                ctx.moveTo(x - r * 0.42, y + r * 0.02);
                ctx.lineTo(x - r * 0.12, y + r * 0.32);
                ctx.lineTo(x + r * 0.45, y - r * 0.3);
                ctx.stroke();
            } else if (status === 'critical') {
                ctx.moveTo(x - r * 0.32, y - r * 0.32);
                ctx.lineTo(x + r * 0.32, y + r * 0.32);
                ctx.moveTo(x + r * 0.32, y - r * 0.32);
                ctx.lineTo(x - r * 0.32, y + r * 0.32);
                ctx.stroke();
            } else if (status === 'warning') {
                ctx.moveTo(x, y - r * 0.45);
                ctx.lineTo(x, y + r * 0.1);
                ctx.stroke();
                ctx.beginPath();
                ctx.arc(x, y + r * 0.42, r * 0.11, 0, 2 * Math.PI);
                ctx.fillStyle = ctx.strokeStyle;
                ctx.fill();
            }
        },

        menuBtnPos(n) {
            return { x: n.x + n.r * 0.72, y: n.y + n.r * 0.72, r: Math.max(n.r * 0.3, 4) };
        },

        nodeHover(n) {
            if (n && !this.nodes.has(n.id)) {
                n = null; // removed by a refresh, but force-graph's hit areas are repainted only periodically
            }
            if (this.pinned && (!n || n.id !== this.pinned)) {
                return;
            }
            this.hover = n ? n.id : null;
            this.updateLabelHover();
            if (n) {
                this.calcHoverRatios();
                this.showMenuBtn(n);
                this.showNodeTooltip(n);
            } else {
                this.tooltip = null;
                this.scheduleMenuBtnClose();
            }
            this.refreshAccessors();
        },

        calcHoverRatios() {
            const hiLinks = this.links.filter((l) => this.isHiLink(l));
            const maxW = max(hiLinks.map((l) => l.weight));
            hiLinks.forEach((l) => (l.hr = maxW > 0 ? l.weight / maxW : 0));
        },

        refreshAccessors() {
            // force-graph evaluates these accessors only when they are set
            this.fg.linkDirectionalParticles(this.fg.linkDirectionalParticles());
            this.fg.linkDirectionalArrowLength(this.fg.linkDirectionalArrowLength());
        },

        refreshHover() {
            if (this.menuNode) {
                const m = this.nodes.get(this.menuNode);
                if (!m) {
                    this.closeMenuBtn();
                } else if (!this.menuOpen()) {
                    this.menuApp = m.app;
                }
            }
            const n = this.hover && this.nodes.get(this.hover);
            if (this.hover && !n) {
                this.pinned = null;
                this.hover = null;
                this.hoverLabel = null;
                this.tooltip = null;
                this.$refs.graph.style.cursor = null;
            } else if (n) {
                this.calcHoverRatios();
                if (this.tooltip && this.tooltip.kind === 'node') {
                    this.showNodeTooltip(n);
                }
            } else if (this.tooltip && this.tooltip.kind === 'link') {
                const l = this.links.find((l) => l.id === this.tooltip.id);
                l ? this.showLinkTooltip(l) : (this.tooltip = null);
            }
            this.refreshAccessors();
        },

        showNodeTooltip(n) {
            const a = n.app;
            this.showTooltip({
                kind: 'node',
                name: n.name,
                ns: (a.labels && a.labels.ns) || '',
                cluster: this.multicluster ? a.cluster : '',
                indicators: (a.indicators || []).map((i) => Object.freeze({ status: i.status, message: i.message })),
            });
        },

        showTooltip(t) {
            this.tooltip = Object.freeze(t);
            this.$nextTick(this.positionTooltip);
        },

        positionTooltip() {
            const el = this.$refs.tooltip && this.$refs.tooltip.$el;
            if (!el || !this.pointer) {
                return;
            }
            const [px, py] = this.pointer;
            let x = px + 14;
            let y = py + 14;
            if (x + el.offsetWidth > this.$el.clientWidth - 4) {
                x = Math.max(4, px - 14 - el.offsetWidth);
            }
            if (y + el.offsetHeight > this.$el.clientHeight - 4) {
                y = Math.max(4, py - 14 - el.offsetHeight);
            }
            el.style.left = x + 'px';
            el.style.top = y + 'px';
        },

        motionChanged(e) {
            this.reducedMotion = e.matches;
            if (this.fg) {
                this.refreshAccessors();
            }
        },

        onLabel(n) {
            const p = this.graphPointer;
            if (!n || !n.labelShown || !p) {
                return false;
            }
            const b = n.labelBox;
            return p.x >= b.x0 && p.x <= b.x1 && p.y >= b.y0 && p.y <= b.y1;
        },

        updateLabelHover() {
            const n = this.hover && this.nodes.get(this.hover);
            const id = this.onLabel(n) ? n.id : null;
            this.$refs.graph.style.cursor = id ? 'pointer' : null;
            if (id !== this.hoverLabel) {
                this.hoverLabel = id;
                this.redraw();
            }
        },

        linkHover(l) {
            if (this.hover) {
                return;
            }
            if (!l) {
                this.tooltip = null;
                return;
            }
            this.showLinkTooltip(l);
        },

        showLinkTooltip(l) {
            this.showTooltip({ kind: 'link', id: l.id, from: l.source.name, to: l.target.name, stats: [...l.stats] });
        },

        nodeClick(n, e) {
            // touch: the first tap selects the node (there is no hover), a tap on its name opens the application
            if (e && e.pointerType === 'touch') {
                if (this.pinned !== n.id || !this.onLabel(n)) {
                    this.pinned = null;
                    this.nodeHover(n);
                    this.pinned = n.id;
                    return;
                }
            } else if (!this.onLabel(n)) {
                return;
            }
            this.openApp(n, e && (e.ctrlKey || e.metaKey));
        },

        openApp(n, newTab) {
            const route = { name: 'overview', params: { view: 'applications', id: n.id }, query: this.$utils.contextQuery() };
            if (newTab) {
                window.open(this.$router.resolve(route).href, '_blank');
                return;
            }
            this.$router.push(route).catch((err) => err);
        },

        // middle-click on the name opens the application in a new tab, like a link (force-graph handles only left clicks)
        auxDown(e) {
            const n = this.hover && this.nodes.get(this.hover);
            if (this.onLabel(n)) {
                e.preventDefault(); // no autoscroll
            }
        },
        auxClick(e) {
            const n = this.hover && this.nodes.get(this.hover);
            if (e.button === 1 && this.onLabel(n)) {
                e.preventDefault();
                this.openApp(n, true);
            }
        },

        contextMenu(e) {
            const n = this.hover && this.nodes.get(this.hover);
            if (!n) {
                return;
            }
            e.preventDefault();
            this.openMenu(n);
        },

        showMenuBtn(n) {
            clearTimeout(this.menuBtnTimer);
            if (this.menuNode === n.id || this.menuOpen()) {
                return;
            }
            this.menuNode = n.id;
            this.menuApp = n.app;
            this.$nextTick(this.placeMenuBtn);
        },

        scheduleMenuBtnClose() {
            clearTimeout(this.menuBtnTimer);
            this.menuBtnTimer = setTimeout(() => {
                if (this.pinned && this.pinned === this.menuNode) {
                    return;
                }
                if (!this.menuBtnHovered && !this.menuOpen()) {
                    this.closeMenuBtn();
                } else if (this.menuOpen()) {
                    this.scheduleMenuBtnClose();
                }
            }, durations.menuClose);
        },

        closeMenuBtn() {
            this.menuNode = null;
            this.menuApp = null;
            this.menuBtnHovered = false;
            this.redraw();
        },

        menuOpen() {
            const b = this.$refs.menuBtn && this.$refs.menuBtn.querySelector('button');
            return !!b && b.getAttribute('aria-expanded') === 'true';
        },

        async openMenu(n) {
            // AppPreferences' menu is opened and closed through its activator button
            const button = () => this.$refs.menuBtn && this.$refs.menuBtn.querySelector('button');
            if (this.menuOpen()) {
                if (this.menuNode === n.id) {
                    return;
                }
                button().click();
                await this.$nextTick();
            }
            this.showMenuBtn(n);
            await this.$nextTick();
            const b = button();
            if (b && this.fg && !this.menuOpen()) {
                b.click();
            }
        },

        placeMenuBtn() {
            const el = this.$refs.menuBtn;
            const n = this.menuNode && this.nodes.get(this.menuNode);
            if (!this.fg || !el || !n || n.x === undefined) {
                return;
            }
            const k = this.fg.zoom();
            const c = this.fg.graph2ScreenCoords(n.x, n.y);
            const size = Math.max(20, 2 * this.menuBtnPos(n).r * k);
            const d = Math.max(0.72 * n.r * k, size / 2 + 6); // never covering the node when zoomed out
            el.style.transform = `translate(${c.x + d - size / 2}px, ${c.y + d - size / 2}px)`;
            el.style.width = el.style.height = size + 'px';
        },

        leave() {
            this.tooltip = null;
            // force-graph keeps the last pointer position when the pointer leaves the canvas: move it out
            const container = this.$refs.graph && this.$refs.graph.querySelector('.force-graph-container');
            if (container && window.PointerEvent) {
                container.dispatchEvent(new PointerEvent('pointermove', { clientX: -1e5, clientY: -1e5 }));
            }
        },

        scheduleResize() {
            clearTimeout(this.resizeTimer);
            this.resizeTimer = setTimeout(this.resize, durations.resize);
        },

        resize() {
            if (!this.fg || !this.$el) {
                return;
            }
            const top = this.$el.getBoundingClientRect().top + window.scrollY;
            const width = this.$el.clientWidth;
            const height = Math.max(400, window.innerHeight - top - 16);
            const [w, h] = [this.fg.width(), this.fg.height()];
            if (width === w && height === h) {
                return;
            }
            const center = this.fg.graphData().nodes.length ? this.fg.screen2GraphCoords(w / 2, h / 2) : null;
            this.fg.width(width).height(height);
            if (center) {
                this.fg.centerAt(center.x, center.y);
            }
        },

        redraw() {
            // force-graph stops rendering once the layout stops; this prop's onChange requests a frame
            // (re-setting the zoom would fire zoom events)
            if (this.fg) {
                this.fg.nodeRelSize(this.fg.nodeRelSize());
            }
        },

        zoomBy(k) {
            this.fitPending = false;
            this.fg.zoom(this.fg.zoom() * k, this.reducedMotion ? 0 : durations.zoom);
        },

        dataApplied() {
            return !this.links.length || typeof this.links[0].source === 'object';
        },

        // like force-graph's zoomToFit, but with a zoom limit (few apps would fill the view) and room for the labels
        fitView(duration) {
            const bbox = this.fg.getGraphBbox();
            if (!bbox) {
                return;
            }
            const p = sizes.fitPadding;
            const k = Math.min(
                (this.fg.width() - 2 * p) / Math.max(bbox.x[1] - bbox.x[0], 1),
                (this.fg.height() - 2 * p - sizes.fitLabel) / Math.max(bbox.y[1] - bbox.y[0], 1),
                zoomLimits.fit,
            );
            this.fg.centerAt((bbox.x[0] + bbox.x[1]) / 2, (bbox.y[0] + bbox.y[1]) / 2 + sizes.fitLabel / 2 / k, duration);
            this.fg.zoom(k, duration);
        },

        fit() {
            this.fitPending = false;
            this.fitView(this.reducedMotion ? 0 : durations.fit);
        },

        // The longest call path to each app from a client; cycles are broken by ignoring DFS back edges.
        calcLevels(nodes, links) {
            const out = new Map();
            nodes.forEach((n, id) => out.set(id, []));
            links.forEach((l) => out.get(l.source).push(l.target));
            const back = new Set();
            const state = new Map(); // 1: on the DFS stack, 2: done
            [...nodes.keys()].sort().forEach((root) => {
                if (state.has(root)) {
                    return;
                }
                const stack = [[root, 0]];
                state.set(root, 1);
                while (stack.length) {
                    const top = stack[stack.length - 1];
                    const next = out.get(top[0])[top[1]++];
                    if (next === undefined) {
                        state.set(top[0], 2);
                        stack.pop();
                    } else if (state.get(next) === 1) {
                        back.add(top[0] + '->' + next);
                    } else if (!state.has(next)) {
                        state.set(next, 1);
                        stack.push([next, 0]);
                    }
                }
            });
            const indeg = new Map();
            nodes.forEach((n, id) => {
                indeg.set(id, 0);
                n.level = 0;
            });
            const dag = links.filter((l) => !back.has(l.id));
            dag.forEach((l) => indeg.set(l.target, indeg.get(l.target) + 1));
            const queue = [...nodes.keys()].filter((id) => !indeg.get(id));
            const succ = new Map();
            dag.forEach((l) => (succ.get(l.source) || succ.set(l.source, []).get(l.source)).push(l.target));
            for (let i = 0; i < queue.length; i++) {
                const id = queue[i];
                const lvl = nodes.get(id).level;
                (succ.get(id) || []).forEach((t) => {
                    const n = nodes.get(t);
                    n.level = Math.max(n.level, lvl + 1);
                    indeg.set(t, indeg.get(t) - 1);
                    if (!indeg.get(t)) {
                        queue.push(t);
                    }
                });
            }
            this.maxLevel = max([...nodes.values()].map((n) => n.level));
            nodes.forEach((n) => !n.deg && (n.level = this.maxLevel));
        },

        applyXForce() {
            if (!this.fg) {
                return;
            }
            if (this.flow) {
                const half = this.maxLevel / 2;
                this.fg.d3Force('x', forceX((n) => (n.level - half) * forces.flowLevelDistance).strength(forces.flowStrength));
            } else {
                this.fg.d3Force('x', forceX(0).strength(forces.centerX));
            }
        },

        relayout() {
            this.releaseSettle();
            this.fg.graphData().nodes.forEach((n) => {
                n.x = n.y = n.vx = n.vy = n.fx = n.fy = undefined;
            });
            this.startLayout();
            this.fg.graphData(this.fg.graphData());
        },

        pointerMove(e) {
            const r = this.$el.getBoundingClientRect();
            this.pointer = [e.clientX - r.left, e.clientY - r.top];
            if (e.type === 'pointermove' && e.buttons) {
                this.fitPending = false;
            }
            if (this.pinned && e.pointerType === 'mouse') {
                this.pinned = null;
            }
            if (this.fg) {
                const g = this.$refs.graph.getBoundingClientRect();
                this.graphPointer = this.fg.screen2GraphCoords(e.clientX - g.left, e.clientY - g.top);
                this.updateLabelHover();
            }
            if (this.tooltip) {
                this.positionTooltip();
            }
        },
    },
};
</script>

<style scoped>
.graph-wrapper {
    position: relative;
    overflow: hidden;
    border: 1px solid var(--border-color);
    border-radius: 4px;
}
.graph.hidden {
    visibility: hidden;
}
.laying-out {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
}
.toolbar {
    position: absolute;
    top: 8px;
    right: 8px;
    display: flex;
    flex-direction: column;
    background-color: var(--background-color);
    border: 1px solid var(--border-color);
    border-radius: 4px;
}
.menu-btn {
    position: absolute;
    top: 0;
    left: 0;
    border-radius: 50%;
    background-color: var(--background-color);
    border: 1px solid var(--border-color);
    display: flex;
    align-items: center;
    justify-content: center;
}
.menu-btn:deep(.v-btn) {
    margin: 0 !important;
}
.tooltip {
    position: absolute;
    z-index: 5;
    pointer-events: none;
    max-width: 360px;
    font-size: 14px;
}
</style>
