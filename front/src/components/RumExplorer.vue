<template>
    <v-card outlined class="mt-6 rum-explorer">
        <v-progress-linear v-if="explorerLoading" indeterminate height="2" color="success" />
        <div class="d-flex align-center flex-wrap pa-3 pb-0" style="gap: 8px">
            <div class="text-subtitle-1 mr-2">Explorer</div>
            <v-tabs v-model="tab" dense background-color="transparent" height="36" class="rum-explorer-tabs">
                <v-tab>Sessions ({{ sessions.length }})</v-tab>
                <v-tab>
                    Page views ({{ pageViews.length }})
                    <span v-if="heatmapHint" class="caption grey--text ml-1">· heatmap</span>
                </v-tab>
            </v-tabs>
        </div>

        <div class="px-3 pt-2">
            <div class="d-flex align-start" style="gap: 8px">
                <v-text-field
                    v-model="draft"
                    dense
                    outlined
                    hide-details="auto"
                    :error-messages="explorerError ? [explorerError] : []"
                    prepend-inner-icon="mdi-filter-variant"
                    placeholder="Filter: errors>0 lcp>4000 browser:Chrome path~checkout"
                    class="rum-explorer-query flex-grow-1"
                    @keydown.enter.prevent="applyDraft"
                    @blur="applyDraft"
                    @click:clear="clearQuery"
                    clearable
                >
                    <template #append>
                        <v-tooltip bottom max-width="360">
                            <template #activator="{ on }">
                                <v-icon small class="mt-1 rum-help" v-on="on">mdi-information-outline</v-icon>
                            </template>
                            <div class="pa-1 caption">
                                <div class="font-weight-medium mb-1">Query syntax</div>
                                <div><code>field:value</code> equal · <code>field~value</code> contains</div>
                                <div><code>field&gt;N</code> / <code>&gt;=</code> / <code>&lt;</code> / <code>&lt;=</code></div>
                                <div>Durations: <code>30s</code>, <code>2.5s</code>, <code>4000ms</code></div>
                                <div>Quotes: <code>path~"my page"</code></div>
                                <div class="mt-1">Sessions: duration, pages, errors, browser, os, device, country, version, session</div>
                                <div>Page views: path, load, lcp, inp, ttfb, cls, trace + shared fields</div>
                            </div>
                        </v-tooltip>
                    </template>
                </v-text-field>
            </div>

            <div v-if="termChips.length" class="d-flex flex-wrap mt-2" style="gap: 6px">
                <v-chip
                    v-for="(c, i) in termChips"
                    :key="i + c.raw"
                    small
                    close
                    :class="{ 'rum-chip-dim': c.dimmed }"
                    :title="c.dimmed ? c.hint : c.raw"
                    @click:close="removeTerm(c.raw)"
                >
                    {{ c.raw }}
                </v-chip>
            </div>

            <div class="d-flex flex-wrap align-center mt-2" style="gap: 6px">
                <v-chip
                    v-for="chip in visibleQuickChips"
                    :key="chip.term"
                    small
                    :outlined="!hasTerm(chip.term)"
                    :color="hasTerm(chip.term) ? 'primary' : undefined"
                    @click="toggleQuick(chip.term)"
                >
                    {{ chip.label }}
                </v-chip>
                <v-spacer />
                <v-btn small text color="primary" @click="moreOpen = !moreOpen">
                    <v-icon left small>{{ moreOpen ? 'mdi-chevron-up' : 'mdi-chevron-down' }}</v-icon>
                    More filters
                </v-btn>
            </div>

            <v-expand-transition>
                <div v-show="moreOpen" class="rum-more mt-3 pa-3">
                    <v-row dense>
                        <v-col v-for="r in rangeFields" :key="r.field + '-min'" :cols="fieldCols">
                            <v-text-field
                                :value="panelRanges[r.field] && panelRanges[r.field].min"
                                :label="r.label + ' min'"
                                dense
                                outlined
                                hide-details
                                type="number"
                                @change="setRange(r.field, 'min', $event)"
                            />
                        </v-col>
                        <v-col v-for="r in rangeFields" :key="r.field + '-max'" :cols="fieldCols">
                            <v-text-field
                                :value="panelRanges[r.field] && panelRanges[r.field].max"
                                :label="r.label + ' max'"
                                dense
                                outlined
                                hide-details
                                type="number"
                                @change="setRange(r.field, 'max', $event)"
                            />
                        </v-col>
                    </v-row>
                    <v-row dense class="mt-1">
                        <v-col :cols="fieldCols">
                            <v-select
                                :value="panelDims.browser"
                                :items="browsers"
                                label="Browser"
                                dense
                                outlined
                                hide-details
                                clearable
                                @change="setDim('browser', $event)"
                            />
                        </v-col>
                        <v-col :cols="fieldCols">
                            <v-select
                                :value="panelDims.os"
                                :items="operatingSystems"
                                label="OS"
                                dense
                                outlined
                                hide-details
                                clearable
                                @change="setDim('os', $event)"
                            />
                        </v-col>
                        <v-col :cols="fieldCols">
                            <v-select
                                :value="panelDims.device"
                                :items="devices"
                                label="Device"
                                dense
                                outlined
                                hide-details
                                clearable
                                @change="setDim('device', $event)"
                            />
                        </v-col>
                        <v-col :cols="fieldCols">
                            <v-select
                                :value="panelDims.country"
                                :items="countries"
                                label="Country"
                                dense
                                outlined
                                hide-details
                                clearable
                                @change="setDim('country', $event)"
                            />
                        </v-col>
                        <v-col :cols="fieldCols">
                            <v-select
                                :value="panelDims.version"
                                :items="versions"
                                label="Version"
                                dense
                                outlined
                                hide-details
                                clearable
                                @change="setDim('version', $event)"
                            />
                        </v-col>
                        <v-col v-if="tab === 1" :cols="fieldCols">
                            <v-text-field
                                :value="panelText.path"
                                label="Path contains"
                                dense
                                outlined
                                hide-details
                                @change="setTextField('path', '~', $event)"
                            />
                        </v-col>
                        <v-col :cols="fieldCols">
                            <v-text-field
                                :value="panelText.session"
                                label="Session id"
                                dense
                                outlined
                                hide-details
                                @change="setTextField('session', ':', $event)"
                            />
                        </v-col>
                        <v-col v-if="tab === 1" :cols="fieldCols">
                            <v-text-field
                                :value="panelText.trace"
                                label="Trace id"
                                dense
                                outlined
                                hide-details
                                @change="setTextField('trace', ':', $event)"
                            />
                        </v-col>
                    </v-row>
                </div>
            </v-expand-transition>

            <div v-if="limitHint" class="caption amber--text text--darken-2 mt-2 mb-1">{{ limitHint }}</div>
        </div>

        <v-tabs-items v-model="tab">
            <v-tab-item>
                <v-data-table
                    dense
                    class="rum-explorer-table rum-explorer-sessions"
                    mobile-breakpoint="0"
                    :headers="visibleSessionHeaders"
                    :items="sessions"
                    sort-by="started_at"
                    sort-desc
                    must-sort
                    :items-per-page="25"
                    :footer-props="{ itemsPerPageOptions: [10, 25, 50] }"
                    no-data-text="No sessions in this range"
                    @click:row="onSessionRow"
                >
                    <template #item.session_id="{ item }">
                        <span class="rum-mono">{{ shortId(item.session_id, 12) }}</span>
                    </template>
                    <template #item.started_at="{ item }">
                        <span class="text-no-wrap">{{ formatTime(item.started_at) }}</span>
                    </template>
                    <template #item.duration_ms="{ item }">
                        <span class="text-no-wrap">{{ formatMs(item.duration_ms) }}</span>
                    </template>
                    <template #item.errors="{ item }">
                        <v-chip v-if="item.errors > 0" x-small color="error" text-color="white">{{ item.errors }}</v-chip>
                        <span v-else>0</span>
                    </template>
                    <template #item.browser="{ item }">{{ item.browser || '—' }}</template>
                    <template #item.os="{ item }">{{ item.os || '—' }}</template>
                    <template #item.device="{ item }">{{ item.device || '—' }}</template>
                    <template #item.country="{ item }">{{ item.country || '—' }}</template>
                </v-data-table>
            </v-tab-item>

            <v-tab-item>
                <v-data-table
                    dense
                    class="rum-explorer-table"
                    mobile-breakpoint="0"
                    :headers="visiblePageViewHeaders"
                    :items="pageViews"
                    sort-by="timestamp"
                    sort-desc
                    must-sort
                    :items-per-page="25"
                    :footer-props="{ itemsPerPageOptions: [10, 25, 50] }"
                    no-data-text="No page views"
                >
                    <template #item.timestamp="{ item }">
                        <span class="text-no-wrap">{{ formatTime(item.timestamp) }}</span>
                    </template>
                    <template #item.page_path="{ item }">
                        <span class="rum-cell-truncate d-inline-block" style="max-width: 180px" :title="item.page_path">{{
                            item.page_path || '—'
                        }}</span>
                    </template>
                    <template #item.duration="{ item }">
                        <span class="text-no-wrap">{{ formatMs(item.duration) }}</span>
                    </template>
                    <template #item.lcp_ms="{ item }">
                        <span class="text-no-wrap" :class="vitalTone(item.lcp_ms, thresholds.lcp_ms)">{{ formatMs(item.lcp_ms) }}</span>
                    </template>
                    <template #item.ttfb_ms="{ item }">
                        <span class="text-no-wrap" :class="vitalTone(item.ttfb_ms, thresholds.ttfb_ms)">{{ formatMs(item.ttfb_ms) }}</span>
                    </template>
                    <template #item.inp_ms="{ item }">
                        <span class="text-no-wrap" :class="vitalTone(item.inp_ms, thresholds.inp_ms)">{{ formatMs(item.inp_ms) }}</span>
                    </template>
                    <template #item.cls="{ item }">
                        <span class="text-no-wrap" :class="vitalTone(item.cls, thresholds.cls, true)">{{ formatCls(item.cls) }}</span>
                    </template>
                    <template #item.browser="{ item }">{{ item.browser || '—' }}</template>
                    <template #item.country="{ item }">{{ item.country || '—' }}</template>
                    <template #item.session_id="{ item }">
                        <a v-if="item.session_id" href="#" @click.prevent="$emit('open-session', item.session_id)">
                            {{ shortId(item.session_id, 8) }}
                        </a>
                        <span v-else>—</span>
                    </template>
                    <template #item.trace_id="{ item }">
                        <a v-if="item.trace_id" href="#" @click.prevent="$emit('open-trace', item.trace_id)">
                            {{ shortId(item.trace_id, 8) }}
                        </a>
                        <span v-else>—</span>
                    </template>
                </v-data-table>
            </v-tab-item>
        </v-tabs-items>
    </v-card>
</template>

<script>
const SESSION_HEADERS = [
    { text: 'Session', value: 'session_id', sortable: false },
    { text: 'Started', value: 'started_at' },
    { text: 'Duration', value: 'duration_ms' },
    { text: 'Pages', value: 'page_views' },
    { text: 'Errors', value: 'errors' },
    { text: 'Browser', value: 'browser' },
    { text: 'OS', value: 'os' },
    { text: 'Device', value: 'device' },
    { text: 'Country', value: 'country' },
];

const PAGE_VIEW_HEADERS = [
    { text: 'Time', value: 'timestamp' },
    { text: 'Path', value: 'page_path' },
    { text: 'Load', value: 'duration' },
    { text: 'LCP', value: 'lcp_ms' },
    { text: 'TTFB', value: 'ttfb_ms' },
    { text: 'INP', value: 'inp_ms' },
    { text: 'CLS', value: 'cls' },
    { text: 'Browser', value: 'browser' },
    { text: 'Country', value: 'country' },
    { text: 'Session', value: 'session_id', sortable: false },
    { text: 'Trace', value: 'trace_id', sortable: false },
];

const COMPACT_SESSION_HIDE = new Set(['os', 'device']);
const COMPACT_PAGEVIEW_HIDE = new Set(['ttfb_ms', 'cls', 'country']);

const SESSION_ONLY = new Set(['duration', 'pages']);
const PAGEVIEW_ONLY = new Set(['path', 'load', 'lcp', 'inp', 'ttfb', 'cls', 'trace']);

const QUICK_CHIPS = [
    { label: 'Errors only', term: 'errors>0', tabs: [0, 1] },
    { label: 'Poor LCP', term: 'lcp>4000', tabs: [1] },
    { label: 'Poor INP', term: 'inp>500', tabs: [1] },
    { label: 'Poor CLS', term: 'cls>0.25', tabs: [1] },
    { label: 'Slow load', term: 'load>3000', tabs: [1] },
];

const RANGE_SESSION = [
    { field: 'duration', label: 'Duration (ms)' },
    { field: 'pages', label: 'Pages' },
];
const RANGE_PAGEVIEW = [
    { field: 'load', label: 'Load (ms)' },
    { field: 'lcp', label: 'LCP (ms)' },
    { field: 'inp', label: 'INP (ms)' },
    { field: 'ttfb', label: 'TTFB (ms)' },
    { field: 'cls', label: 'CLS' },
];

function tokenizeClient(s) {
    const out = [];
    if (!s) return out;
    let cur = '';
    let inQuote = false;
    for (let i = 0; i < s.length; i++) {
        const c = s[i];
        if (c === '\\' && i + 1 < s.length) {
            cur += c + s[++i];
            continue;
        }
        if (c === '"') {
            inQuote = !inQuote;
            cur += c;
            continue;
        }
        if (/\s/.test(c) && !inQuote) {
            if (cur) out.push(cur);
            cur = '';
            continue;
        }
        cur += c;
    }
    if (cur) out.push(cur);
    return out;
}

function parseTokenClient(tok) {
    const m = tok.match(/^([a-zA-Z_][a-zA-Z0-9_]*)(:|~|>=|<=|>|<)(.+)$/);
    if (!m) return { field: '', op: '', value: tok, raw: tok };
    return { field: m[1].toLowerCase(), op: m[2], value: m[3].replace(/^"|"$/g, ''), raw: tok };
}

function quoteIfNeeded(v) {
    const s = String(v);
    if (!s) return '';
    if (/\s/.test(s) || /["']/.test(s)) return `"${s.replace(/"/g, '\\"')}"`;
    return s;
}

function rebuildQuery(tokens) {
    return tokens.filter(Boolean).join(' ');
}

export default {
    props: {
        sessions: { type: Array, default: () => [] },
        pageViews: { type: Array, default: () => [] },
        thresholds: { type: Object, default: () => ({}) },
        heatmapHint: { type: Boolean, default: false },
        layout: { type: String, default: 'regular' },
        query: { type: String, default: '' },
        explorerError: { type: String, default: '' },
        explorerLoading: { type: Boolean, default: false },
        sessionsLimited: { type: Boolean, default: false },
        pageViewsLimited: { type: Boolean, default: false },
        browsers: { type: Array, default: () => [] },
        operatingSystems: { type: Array, default: () => [] },
        devices: { type: Array, default: () => [] },
        countries: { type: Array, default: () => [] },
        versions: { type: Array, default: () => [] },
    },
    data() {
        return {
            tab: 0,
            draft: this.query || '',
            moreOpen: false,
        };
    },
    watch: {
        query(v) {
            if (String(v || '') !== String(this.draft || '').trim()) {
                this.draft = v || '';
            }
        },
    },
    computed: {
        fieldCols() {
            if (this.layout === 'compact') return 6;
            if (this.layout === 'wide') return 2;
            return 3;
        },
        tokens() {
            return tokenizeClient(this.query).map(parseTokenClient);
        },
        termChips() {
            const sessionTab = this.tab === 0;
            return this.tokens.map((t) => {
                let dimmed = false;
                let hint = '';
                if (t.field) {
                    if (sessionTab && PAGEVIEW_ONLY.has(t.field)) {
                        dimmed = true;
                        hint = 'not applied to Sessions';
                    }
                    if (!sessionTab && SESSION_ONLY.has(t.field)) {
                        dimmed = true;
                        hint = 'not applied to Page views';
                    }
                }
                return { raw: t.raw, dimmed, hint };
            });
        },
        visibleQuickChips() {
            return QUICK_CHIPS.filter((c) => c.tabs.includes(this.tab));
        },
        rangeFields() {
            return this.tab === 0 ? RANGE_SESSION : RANGE_PAGEVIEW;
        },
        panelRanges() {
            const out = {};
            this.tokens.forEach((t) => {
                if (!t.field || !['>', '>=', '<', '<='].includes(t.op)) return;
                if (!out[t.field]) out[t.field] = {};
                const n = parseFloat(String(t.value).replace(/(ms|s)$/i, ''));
                if (Number.isNaN(n)) return;
                if (t.op === '>' || t.op === '>=') out[t.field].min = n;
                if (t.op === '<' || t.op === '<=') out[t.field].max = n;
            });
            return out;
        },
        panelDims() {
            const out = { browser: null, os: null, device: null, country: null, version: null };
            this.tokens.forEach((t) => {
                if (t.op === ':' && Object.prototype.hasOwnProperty.call(out, t.field)) {
                    out[t.field] = t.value;
                }
            });
            return out;
        },
        panelText() {
            const out = { path: '', session: '', trace: '' };
            this.tokens.forEach((t) => {
                if (t.field === 'path' && t.op === '~') out.path = t.value;
                if (t.field === 'session' && (t.op === ':' || t.op === '~')) out.session = t.value;
                if (t.field === 'trace' && (t.op === ':' || t.op === '~')) out.trace = t.value;
            });
            return out;
        },
        limitHint() {
            if (this.tab === 0 && this.sessionsLimited) return 'Showing first 200 sessions — refine the filter';
            if (this.tab === 1 && this.pageViewsLimited) return 'Showing first 200 page views — refine the filter';
            return '';
        },
        visibleSessionHeaders() {
            if (this.layout !== 'compact') return SESSION_HEADERS;
            return SESSION_HEADERS.filter((h) => !COMPACT_SESSION_HIDE.has(h.value));
        },
        visiblePageViewHeaders() {
            if (this.layout !== 'compact') return PAGE_VIEW_HEADERS;
            return PAGE_VIEW_HEADERS.filter((h) => !COMPACT_PAGEVIEW_HIDE.has(h.value));
        },
    },
    methods: {
        applyDraft() {
            const next = String(this.draft || '').trim();
            if (next === String(this.query || '').trim()) return;
            this.$emit('update-query', next);
        },
        clearQuery() {
            this.draft = '';
            this.$emit('update-query', '');
        },
        hasTerm(term) {
            return this.tokens.some((t) => t.raw === term);
        },
        toggleQuick(term) {
            const toks = tokenizeClient(this.query);
            const idx = toks.indexOf(term);
            if (idx >= 0) toks.splice(idx, 1);
            else toks.push(term);
            this.$emit('update-query', rebuildQuery(toks));
        },
        removeTerm(raw) {
            const toks = tokenizeClient(this.query).filter((t) => t !== raw);
            this.$emit('update-query', rebuildQuery(toks));
        },
        rewriteField(field, builder) {
            const toks = tokenizeClient(this.query).filter((t) => {
                const p = parseTokenClient(t);
                return p.field !== field;
            });
            const extras = builder() || [];
            extras.forEach((e) => {
                if (e) toks.push(e);
            });
            this.$emit('update-query', rebuildQuery(toks));
        },
        setRange(field, which, value) {
            const cur = { ...(this.panelRanges[field] || {}) };
            const v = value === '' || value == null ? null : Number(value);
            if (which === 'min') cur.min = v;
            else cur.max = v;
            this.rewriteField(field, () => {
                const out = [];
                if (cur.min != null && !Number.isNaN(cur.min)) out.push(`${field}>=${cur.min}`);
                if (cur.max != null && !Number.isNaN(cur.max)) out.push(`${field}<=${cur.max}`);
                return out;
            });
        },
        setDim(field, value) {
            this.rewriteField(field, () => {
                if (value == null || value === '') return [];
                return [`${field}:${quoteIfNeeded(value)}`];
            });
        },
        setTextField(field, op, value) {
            this.rewriteField(field, () => {
                const v = String(value || '').trim();
                if (!v) return [];
                return [`${field}${op}${quoteIfNeeded(v)}`];
            });
        },
        shortId(id, n) {
            if (!id) return '—';
            return String(id).substring(0, n);
        },
        formatTime(ts) {
            if (!ts) return '—';
            return new Date(ts).toLocaleTimeString();
        },
        formatMs(v) {
            if (v == null || v === 0 || Number.isNaN(Number(v))) return '—';
            return Math.round(Number(v)) + ' ms';
        },
        formatCls(v) {
            if (v == null || Number.isNaN(Number(v))) return '—';
            if (v === 0) return '0';
            return Number(v).toFixed(3);
        },
        vitalPoorCutoff(threshold, cls) {
            if (threshold == null) return null;
            if (cls) return 0.25;
            if (threshold <= 200) return 500;
            if (threshold <= 800) return 1800;
            if (threshold <= 2500) return 4000;
            return threshold * 1.6;
        },
        vitalTone(value, threshold, cls) {
            if (value == null || threshold == null) return '';
            const poor = this.vitalPoorCutoff(threshold, cls);
            if (value <= threshold) return 'success--text';
            if (value <= poor) return 'warning--text';
            return 'error--text';
        },
        onSessionRow(item) {
            if (item && item.session_id) {
                this.$emit('open-session', item.session_id);
            }
        },
    },
};
</script>

<style scoped>
.rum-explorer-tabs {
    flex: 0 1 auto;
}
.rum-explorer-sessions >>> tbody tr {
    cursor: pointer;
}
.rum-mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 12px;
}
.rum-cell-truncate {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    vertical-align: bottom;
}
.rum-chip-dim {
    opacity: 0.45;
}
.rum-more {
    background: rgba(0, 0, 0, 0.03);
    border-radius: 4px;
}
.rum-help {
    opacity: 0.6;
    cursor: help;
}
.rum-help:hover {
    opacity: 1;
}
</style>
