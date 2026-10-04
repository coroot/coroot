<template>
    <v-navigation-drawer :value="!!sessionId" fixed right temporary :width="panelWidth" style="z-index: 10" @input="onDrawerInput">
        <div class="pa-4 d-flex flex-column" style="min-height: 100%">
            <div class="d-flex align-center mb-2">
                <div class="text-h6">Session</div>
                <v-spacer />
                <v-btn icon small @click="close"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <div class="d-flex align-center mb-3" style="gap: 4px">
                <div class="caption rum-mono text-break">{{ sessionId }}</div>
                <v-btn icon x-small @click="copyId" :title="'Copy session id'">
                    <v-icon x-small>mdi-content-copy</v-icon>
                </v-btn>
            </div>

            <v-progress-linear v-if="loading" indeterminate height="2" color="success" class="mb-3" />

            <template v-if="info">
                <div class="d-flex flex-wrap mb-3" style="gap: 6px">
                    <v-chip v-if="info.browser" x-small outlined>{{ info.browser }}</v-chip>
                    <v-chip v-if="info.os" x-small outlined>{{ info.os }}</v-chip>
                    <v-chip v-if="info.device" x-small outlined>{{ info.device }}</v-chip>
                    <v-chip v-if="info.country" x-small outlined>{{ info.country }}</v-chip>
                    <v-chip v-if="info.version" x-small outlined>v{{ info.version }}</v-chip>
                </div>
                <div class="d-flex flex-wrap caption grey--text mb-4" style="gap: 12px">
                    <span v-if="info.started_at">Started {{ formatTime(info.started_at) }}</span>
                    <span>Duration {{ formatMs(info.duration_ms) }}</span>
                    <span>{{ info.page_views || 0 }} pages</span>
                    <span :class="info.errors > 0 ? 'error--text' : ''">{{ info.errors || 0 }} errors</span>
                </div>
            </template>

            <div v-if="!loading && !groups.length" class="grey--text">No events in this session</div>

            <div v-for="(g, gi) in groups" :key="gi" class="mb-4">
                <div class="d-flex align-center mb-1" style="gap: 8px">
                    <v-icon small color="primary">mdi-web</v-icon>
                    <div class="font-weight-medium text-truncate" style="max-width: 280px">{{ g.path || '—' }}</div>
                    <span class="caption grey--text text-no-wrap">{{ formatTime(g.timestamp) }}</span>
                    <v-spacer />
                    <span v-if="g.duration_ms" class="caption text-no-wrap">{{ formatMs(g.duration_ms) }}</span>
                    <a v-if="g.trace_id" href="#" class="caption" @click.prevent.stop="openTrace(g.trace_id)">trace</a>
                </div>
                <div class="ml-6">
                    <div v-for="(ev, ei) in g.events" :key="ei" class="d-flex align-start py-1 rum-event-row" style="gap: 8px">
                        <v-icon x-small :color="eventColor(ev)">{{ eventIcon(ev) }}</v-icon>
                        <div class="flex-grow-1" style="min-width: 0">
                            <div class="d-flex align-center" style="gap: 6px">
                                <span class="caption grey--text text-no-wrap">{{ formatTime(ev.timestamp) }}</span>
                                <span class="text-body-2 text-truncate">{{ ev.name }}</span>
                                <v-chip v-if="isError(ev)" x-small color="error" text-color="white">error</v-chip>
                            </div>
                            <div v-if="ev.page_path && ev.page_path !== g.path" class="caption grey--text text-truncate">
                                {{ ev.page_path }}
                            </div>
                        </div>
                        <span v-if="showDuration(ev)" class="caption text-no-wrap">{{ formatMs(ev.duration_ms) }}</span>
                        <a v-if="ev.trace_id" href="#" class="caption" @click.prevent.stop="openTrace(ev.trace_id)">trace</a>
                    </div>
                    <div v-if="!g.events.length" class="caption grey--text">No nested events</div>
                </div>
            </div>
        </div>
    </v-navigation-drawer>
</template>

<script>
const PAGE_ROOT_NAMES = new Set(['documentLoad', 'routeChange', 'documentFetch']);
const VITAL_NAMES = new Set(['lcp', 'ttfb', 'cls', 'inp', 'fcp', 'fid']);

export default {
    props: {
        appId: String,
        sessionId: String,
        layout: { type: String, default: 'regular' },
    },
    data() {
        return {
            loading: false,
            info: null,
            events: [],
            reqSeq: 0,
        };
    },
    computed: {
        panelWidth() {
            if (this.layout === 'wide') return 640;
            const vw = typeof window !== 'undefined' ? window.innerWidth : 560;
            return Math.min(560, Math.round(vw * 0.92));
        },
        groups() {
            const events = this.events || [];
            if (!events.length) return [];
            const groups = [];
            let current = null;
            const startGroup = (ev) => {
                current = {
                    path: ev.page_path || '',
                    timestamp: ev.timestamp,
                    duration_ms: ev.kind === 'span' ? ev.duration_ms : 0,
                    trace_id: ev.trace_id || '',
                    events: [],
                };
                groups.push(current);
            };
            for (const ev of events) {
                const isRoot =
                    ev.kind === 'span' &&
                    (PAGE_ROOT_NAMES.has(ev.name) || (ev.page_path && !VITAL_NAMES.has(ev.name) && !ev.name.startsWith('HTTP')));
                // Prefer explicit page-navigation spans as group roots.
                if (ev.kind === 'span' && PAGE_ROOT_NAMES.has(ev.name)) {
                    startGroup(ev);
                    continue;
                }
                if (!current) {
                    startGroup(ev);
                    if (PAGE_ROOT_NAMES.has(ev.name)) continue;
                }
                // New page path without a navigation span → start a soft group.
                if (ev.page_path && current.path && ev.page_path !== current.path && isRoot) {
                    startGroup(ev);
                    continue;
                }
                current.events.push(ev);
            }
            return groups;
        },
    },
    watch: {
        sessionId: {
            immediate: true,
            handler(id) {
                if (id) this.fetchSession(id);
                else {
                    this.info = null;
                    this.events = [];
                }
            },
        },
    },
    methods: {
        onDrawerInput(open) {
            // Only treat as a user dismiss (overlay/Esc). If the parent already cleared
            // sessionId (e.g. navigating to a trace), do not emit close — that would
            // race and wipe rum_trace from the query.
            if (!open && this.sessionId) this.close();
        },
        close() {
            this.$emit('close');
        },
        openTrace(id) {
            if (!id) return;
            this.$emit('open-trace', id);
        },
        copyId() {
            if (!this.sessionId || !navigator.clipboard) return;
            navigator.clipboard.writeText(this.sessionId).catch(() => {});
        },
        formatTime(ts) {
            if (!ts) return '—';
            return new Date(ts).toLocaleTimeString();
        },
        formatMs(v) {
            if (v == null || v === 0 || Number.isNaN(Number(v))) return '—';
            return Math.round(Number(v)) + ' ms';
        },
        isError(ev) {
            if (!ev) return false;
            if (ev.name === 'js_error' || ev.name === 'window.onerror' || ev.name === 'unhandledrejection') return true;
            const s = String(ev.status || '').toLowerCase();
            return s.includes('error') || s === 'poor';
        },
        showDuration(ev) {
            if (!ev || ev.duration_ms == null || ev.duration_ms === 0) return false;
            if (ev.name === 'cls') return false;
            return true;
        },
        eventIcon(ev) {
            if (this.isError(ev)) return 'mdi-alert-circle-outline';
            if (ev.kind === 'event' && VITAL_NAMES.has(ev.name)) return 'mdi-speedometer';
            if (ev.name && String(ev.name).startsWith('HTTP')) return 'mdi-api';
            if (ev.kind === 'event') return 'mdi-flash-outline';
            return 'mdi-circle-small';
        },
        eventColor(ev) {
            if (this.isError(ev)) return 'error';
            if (ev.kind === 'event' && VITAL_NAMES.has(ev.name)) return 'primary';
            if (ev.name && String(ev.name).startsWith('HTTP')) return 'info';
            return 'grey';
        },
        fetchSession(id) {
            const seq = ++this.reqSeq;
            this.loading = true;
            this.info = null;
            this.events = [];
            this.$api.getRum(this.appId, { only: 'session', session: id }, (data, error) => {
                if (seq !== this.reqSeq) return;
                this.loading = false;
                if (error) {
                    this.events = [];
                    return;
                }
                this.info = (data && data.session_info) || null;
                this.events = (data && data.session) || [];
            });
        },
    },
};
</script>

<style scoped>
.rum-mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.rum-event-row:hover {
    background: rgba(0, 0, 0, 0.03);
}
</style>
