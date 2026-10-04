<template>
    <div v-on-resize="onContentResize" class="rum-app">
        <v-progress-linear v-if="loading && isRumClient && !showStatusCard" indeterminate height="2" color="success" class="mt-4" />

        <!-- Context strip -->
        <div class="d-flex align-center flex-wrap mt-4 mb-2" style="gap: 8px">
            <template v-if="showStatusCard">
                <Led :status="view.status" />
                <span v-html="view.message" />
                <RumIntegration v-if="view.status !== 'ok'" small color="primary">Integrate RUM</RumIntegration>
            </template>
            <template v-else>
                <span class="caption grey--text" v-if="view.message" v-html="view.message" />
            </template>
            <span v-if="view.raw_retention" class="caption grey--text text--darken-1">
                Detailed data (sessions, page views) kept {{ view.raw_retention }}; dashboards up to
                {{ view.aggregates_retention || view.raw_retention }}
            </span>
            <v-spacer />
            <v-select
                v-if="!isRumClient && view.services && view.services.length"
                :value="service"
                :items="services"
                @change="changeService"
                outlined
                hide-details
                dense
                :menu-props="{ offsetY: true }"
                style="max-width: 260px"
                label="RUM service"
            />
            <v-select
                :value="percentile"
                :items="percentileItems"
                @change="setFilter('percentile', $event)"
                outlined
                hide-details
                dense
                :menu-props="{ offsetY: true }"
                style="max-width: 120px"
                label="Percentile"
            />
        </div>

        <!-- KPI: wide = one row of 7 (equal flex cols) -->
        <v-row v-if="view.summary && layout === 'wide'" dense class="mb-2 rum-kpi-wide">
            <v-col v-for="card in summaryCards" :key="card.key" class="rum-kpi-flex">
                <v-card outlined class="pa-3 fill-height">
                    <div class="caption grey--text d-flex align-center">
                        <span>{{ card.label }}</span>
                        <v-spacer />
                        <v-tooltip bottom max-width="280">
                            <template #activator="{ on }">
                                <v-icon x-small class="rum-kpi-info ml-1" v-on="on">mdi-information-outline</v-icon>
                            </template>
                            <div class="pa-1">{{ card.hint }}</div>
                        </v-tooltip>
                    </div>
                    <div class="text-h6">{{ card.value }}</div>
                    <div v-if="card.delta != null" class="caption" :class="deltaClass(card.delta)">{{ formatDelta(card.delta) }}</div>
                    <div v-if="card.sub" class="caption grey--text">{{ card.sub }}</div>
                </v-card>
            </v-col>
            <v-col v-for="kpi in cwvKpis" :key="kpi.key" class="rum-kpi-flex">
                <v-card outlined class="pa-3 fill-height">
                    <div class="caption grey--text d-flex align-center">
                        <span>{{ kpi.label }} · {{ pctLabel }}</span>
                        <v-spacer />
                        <v-tooltip bottom max-width="300">
                            <template #activator="{ on }">
                                <v-icon x-small class="rum-kpi-info ml-1" v-on="on">mdi-information-outline</v-icon>
                            </template>
                            <div class="pa-1">
                                <div>{{ kpi.hint }}</div>
                                <div v-if="kpi.bandHint" class="caption mt-1">{{ kpi.bandHint }}</div>
                            </div>
                        </v-tooltip>
                        <Led v-if="percentile === 75 && kpi.status" :status="kpi.status" class="ml-1" />
                    </div>
                    <div class="text-h6" :class="kpi.tone">{{ kpi.display }}</div>
                    <div class="caption" :class="deltaClass(kpi.delta)">{{ formatDelta(kpi.delta, kpi.cls) }}</div>
                    <RumCwvBar :kpi="kpi" :format-count="formatCount" />
                    <div v-if="percentile !== 75" class="caption grey--text mt-1">Thresholds apply at p75</div>
                </v-card>
            </v-col>
        </v-row>

        <!-- KPI: regular/compact — cols driven by layout tier, not window breakpoints -->
        <template v-else-if="view.summary">
            <v-row dense class="mb-2">
                <v-col v-for="card in summaryCards" :key="card.key" :cols="layout === 'compact' ? 6 : 4">
                    <v-card outlined class="pa-3 fill-height">
                        <div class="caption grey--text d-flex align-center">
                            <span>{{ card.label }}</span>
                            <v-spacer />
                            <v-tooltip bottom max-width="280">
                                <template #activator="{ on }">
                                    <v-icon x-small class="rum-kpi-info ml-1" v-on="on">mdi-information-outline</v-icon>
                                </template>
                                <div class="pa-1">{{ card.hint }}</div>
                            </v-tooltip>
                        </div>
                        <div class="text-h6">{{ card.value }}</div>
                        <div v-if="card.delta != null" class="caption" :class="deltaClass(card.delta)">{{ formatDelta(card.delta) }}</div>
                        <div v-if="card.sub" class="caption grey--text">{{ card.sub }}</div>
                    </v-card>
                </v-col>
            </v-row>
            <v-row dense class="mb-2">
                <v-col v-for="kpi in cwvKpis" :key="kpi.key" :cols="layout === 'compact' ? 6 : 3">
                    <v-card outlined class="pa-3 fill-height">
                        <div class="caption grey--text d-flex align-center">
                            <span>{{ kpi.label }} · {{ pctLabel }}</span>
                            <v-spacer />
                            <v-tooltip bottom max-width="300">
                                <template #activator="{ on }">
                                    <v-icon x-small class="rum-kpi-info ml-1" v-on="on">mdi-information-outline</v-icon>
                                </template>
                                <div class="pa-1">
                                    <div>{{ kpi.hint }}</div>
                                    <div v-if="kpi.bandHint" class="caption mt-1">{{ kpi.bandHint }}</div>
                                </div>
                            </v-tooltip>
                            <Led v-if="percentile === 75 && kpi.status" :status="kpi.status" class="ml-1" />
                        </div>
                        <div class="text-h6" :class="kpi.tone">{{ kpi.display }}</div>
                        <div class="caption" :class="deltaClass(kpi.delta)">{{ formatDelta(kpi.delta, kpi.cls) }}</div>
                        <RumCwvBar :kpi="kpi" :format-count="formatCount" />
                        <div v-if="percentile !== 75" class="caption grey--text mt-1">Thresholds apply at p75</div>
                    </v-card>
                </v-col>
            </v-row>
        </template>

        <!-- Filter bar -->
        <v-card outlined class="pa-3 mb-4 rum-filters" :style="{ position: 'sticky', top: filterBarTop + 'px', zIndex: 2 }">
            <div class="d-flex flex-wrap align-center" style="gap: 8px">
                <v-combobox
                    :value="filters.page || null"
                    :items="pageFilterItems"
                    @change="setFilter('page', $event)"
                    label="Page"
                    dense
                    outlined
                    hide-details
                    clearable
                    style="max-width: 220px; min-width: 140px"
                />
                <v-select
                    :value="filters.browser || null"
                    :items="nameItems(view.browsers)"
                    @change="setFilter('browser', $event)"
                    label="Browser"
                    dense
                    outlined
                    hide-details
                    clearable
                    style="max-width: 140px; min-width: 110px"
                />
                <template v-if="layout !== 'compact'">
                    <v-select
                        :value="filters.os || null"
                        :items="nameItems(view.operating_systems)"
                        @change="setFilter('os', $event)"
                        label="OS"
                        dense
                        outlined
                        hide-details
                        clearable
                        style="max-width: 130px"
                    />
                    <v-select
                        :value="filters.device || null"
                        :items="nameItems(view.devices)"
                        @change="setFilter('device', $event)"
                        label="Device"
                        dense
                        outlined
                        hide-details
                        clearable
                        style="max-width: 130px"
                    />
                    <v-select
                        :value="filters.country || null"
                        :items="nameItems(view.countries)"
                        @change="setFilter('country', $event)"
                        label="Country"
                        dense
                        outlined
                        hide-details
                        clearable
                        style="max-width: 140px"
                    />
                    <v-select
                        :value="filters.version || null"
                        :items="versionItems"
                        @change="setFilter('version', $event)"
                        label="Version"
                        dense
                        outlined
                        hide-details
                        clearable
                        style="max-width: 140px"
                    />
                </template>
                <v-menu v-else offset-y :close-on-content-click="false">
                    <template #activator="{ on, attrs }">
                        <v-btn outlined dense small v-bind="attrs" v-on="on" color="primary">
                            Filters{{ moreFiltersCount ? ` (${moreFiltersCount})` : '' }}
                            <v-icon small right>mdi-chevron-down</v-icon>
                        </v-btn>
                    </template>
                    <v-card class="pa-3" style="min-width: 220px">
                        <v-select
                            :value="filters.os || null"
                            :items="nameItems(view.operating_systems)"
                            @change="setFilter('os', $event)"
                            label="OS"
                            dense
                            outlined
                            hide-details
                            clearable
                            class="mb-2"
                        />
                        <v-select
                            :value="filters.device || null"
                            :items="nameItems(view.devices)"
                            @change="setFilter('device', $event)"
                            label="Device"
                            dense
                            outlined
                            hide-details
                            clearable
                            class="mb-2"
                        />
                        <v-select
                            :value="filters.country || null"
                            :items="nameItems(view.countries)"
                            @change="setFilter('country', $event)"
                            label="Country"
                            dense
                            outlined
                            hide-details
                            clearable
                            class="mb-2"
                        />
                        <v-select
                            :value="filters.version || null"
                            :items="versionItems"
                            @change="setFilter('version', $event)"
                            label="Version"
                            dense
                            outlined
                            hide-details
                            clearable
                        />
                    </v-card>
                </v-menu>
                <v-btn v-if="hasFilters" text small color="primary" @click="clearFilters">Clear</v-btn>
            </div>
        </v-card>

        <div v-if="traceId" class="mt-5">
            <div class="text-md-h6 mb-3">
                <a href="#" @click.prevent="openTrace('')"><v-icon>mdi-arrow-left</v-icon></a>
                Trace {{ traceId }}
            </div>
            <TracingTrace v-if="view.spans" :spans="view.spans" :span="''" />
        </div>

        <template v-else>
            <!-- Charts 2x2 (rate|errors, then cwv|cls); compact stacks CWV/CLS -->
            <v-row dense>
                <v-col :cols="6" v-for="(ch, i) in rateCharts" :key="'r' + i">
                    <Chart :chart="ch" :selection="{}" />
                </v-col>
            </v-row>
            <v-row dense class="mt-2">
                <v-col v-for="(ch, i) in cwvCharts" :key="'c' + i" :cols="layout === 'compact' ? 12 : 6">
                    <Chart :chart="ch" :selection="{}" />
                </v-col>
            </v-row>

            <Heatmap
                v-if="view.heatmap"
                :heatmap="view.heatmap"
                :selection="heatmapSelection"
                @select="setHeatmapSelection"
                :loading="loading"
                class="mt-5"
            />

            <!-- Tables: wide = 2-col grid; otherwise stacked -->
            <v-row dense class="mt-4">
                <v-col :cols="layout === 'wide' ? 6 : 12">
                    <div class="text-subtitle-1 mb-2">Pages</div>
                    <v-simple-table dense>
                        <thead>
                            <tr>
                                <th style="cursor: pointer" @click="sortBy('path')">Path</th>
                                <th style="cursor: pointer" @click="sortBy('count')">Views</th>
                                <th style="cursor: pointer" @click="sortBy('lcp_ms')">LCP {{ pctLabel }}</th>
                                <th style="cursor: pointer" @click="sortBy('inp_ms')">INP {{ pctLabel }}</th>
                                <th style="cursor: pointer" @click="sortBy('latency_ms')">Load {{ pctLabel }}</th>
                                <th style="cursor: pointer" @click="sortBy('error_count')">Errors</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr
                                v-for="p in sortedPages"
                                :key="p.path"
                                style="cursor: pointer"
                                @click="toggleFilter('page', p.path)"
                                :class="{ 'rum-row-selected': filters.page === p.path }"
                            >
                                <td class="rum-cell-truncate" :title="p.path">{{ p.path }}</td>
                                <td>{{ p.count }}</td>
                                <td class="text-no-wrap">{{ formatMs(p.lcp_ms) }}</td>
                                <td class="text-no-wrap">{{ formatMs(p.inp_ms) }}</td>
                                <td class="text-no-wrap">{{ formatMs(p.latency_ms) }}</td>
                                <td>{{ p.error_count }}</td>
                            </tr>
                            <tr v-if="!(view.top_pages || []).length">
                                <td colspan="6" class="grey--text">No pages in this range</td>
                            </tr>
                        </tbody>
                    </v-simple-table>
                </v-col>

                <v-col :cols="layout === 'wide' ? 6 : 12" v-if="(view.errors || []).length || layout === 'wide'">
                    <div class="text-subtitle-1 mb-2" :class="{ 'mt-6': layout !== 'wide' }">Errors</div>
                    <v-simple-table dense>
                        <thead>
                            <tr>
                                <th>Message</th>
                                <th>Type</th>
                                <th>Count</th>
                                <th>Sessions</th>
                                <th>Top page</th>
                                <th>Trace</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="(e, i) in view.errors || []" :key="i">
                                <td class="rum-cell-truncate" :title="e.message">{{ e.message }}</td>
                                <td>{{ e.type }}</td>
                                <td>{{ e.count }}</td>
                                <td>{{ e.sessions }}</td>
                                <td class="rum-cell-truncate" :title="e.top_page || ''">
                                    <a v-if="e.top_page" href="#" @click.prevent="toggleFilter('page', e.top_page)">{{ e.top_page }}</a>
                                    <span v-else>—</span>
                                </td>
                                <td>
                                    <a v-if="e.sample_trace" href="#" @click.prevent="openTrace(e.sample_trace)">{{
                                        e.sample_trace.substring(0, 8)
                                    }}</a>
                                    <span v-else>—</span>
                                </td>
                            </tr>
                            <tr v-if="!(view.errors || []).length">
                                <td colspan="6" class="grey--text">No errors</td>
                            </tr>
                        </tbody>
                    </v-simple-table>
                </v-col>
            </v-row>

            <v-row dense class="mt-2">
                <v-col :cols="layout === 'wide' ? 6 : 12" v-if="(view.ajax || []).length">
                    <div class="text-subtitle-1 mb-2 mt-4">AJAX / fetch</div>
                    <v-simple-table dense>
                        <thead>
                            <tr>
                                <th>Method</th>
                                <th>URL</th>
                                <th>Calls</th>
                                <th>{{ pctLabel }}</th>
                                <th>Errors</th>
                                <th>Backend</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="(a, i) in view.ajax" :key="i">
                                <td>{{ a.method }}</td>
                                <td class="rum-cell-truncate" :title="a.url">{{ a.url }}</td>
                                <td>{{ a.count }}</td>
                                <td class="text-no-wrap">{{ formatMs(a.latency_ms) }}</td>
                                <td>{{ a.error_count }} ({{ formatPct((a.error_count / (a.count || 1)) * 100) }})</td>
                                <td>{{ a.backend_name || a.peer_service || '—' }}</td>
                            </tr>
                        </tbody>
                    </v-simple-table>
                </v-col>

                <v-col :cols="layout === 'wide' ? 6 : 12">
                    <div class="text-subtitle-1 mb-2 mt-4">Breakdown</div>
                    <div class="d-flex align-center mb-2 flex-wrap" style="gap: 8px">
                        <v-chip-group v-model="breakdownTab" mandatory active-class="primary--text">
                            <v-chip small filter value="browser">Browser</v-chip>
                            <v-chip small filter value="os">OS</v-chip>
                            <v-chip small filter value="device">Device</v-chip>
                            <v-chip small filter value="country">Country</v-chip>
                            <v-chip small filter value="version">Version</v-chip>
                        </v-chip-group>
                        <v-chip v-if="breakdownFilterValue" small close color="primary" text-color="white" @click:close="clearBreakdownFilter">
                            {{ breakdownLabel }}: {{ breakdownFilterValue }}
                        </v-chip>
                    </div>
                    <v-simple-table dense>
                        <thead>
                            <tr>
                                <th>{{ breakdownLabel }}</th>
                                <th>Count</th>
                                <th>{{ pctLabel }}</th>
                                <th>Errors</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr
                                v-for="row in breakdownRows"
                                :key="row.name"
                                style="cursor: pointer"
                                @click="applyBreakdownFilter(row.name)"
                                :class="{ 'rum-row-selected': breakdownFilterValue === row.name }"
                            >
                                <td>{{ row.name }}</td>
                                <td>{{ row.count }}</td>
                                <td class="text-no-wrap">{{ formatMs(row.latency_ms) }}</td>
                                <td>{{ row.error_count || 0 }}</td>
                            </tr>
                            <tr v-if="!breakdownRows.length">
                                <td colspan="4" class="grey--text">No data</td>
                            </tr>
                        </tbody>
                    </v-simple-table>
                </v-col>
            </v-row>

            <RumExplorer
                :sessions="view.sessions || []"
                :page-views="view.page_views || []"
                :thresholds="view.thresholds || {}"
                :heatmap-hint="hasHeatmapSelection"
                :layout="layout"
                :query="rumQuery"
                :explorer-error="view.explorer_error || ''"
                :explorer-loading="explorerLoading"
                :sessions-limited="!!view.sessions_limited"
                :page-views-limited="!!view.page_views_limited"
                :browsers="nameItems(view.browsers)"
                :operating-systems="nameItems(view.operating_systems)"
                :devices="nameItems(view.devices)"
                :countries="nameItems(view.countries)"
                :versions="(view.versions || []).map((x) => x.version).filter(Boolean)"
                @update-query="setRumQuery"
                @open-session="openSession"
                @open-trace="openTrace"
            />
        </template>

        <RumSessionPanel :app-id="appId" :session-id="sessionId" :layout="layout" @close="openSession('')" @open-trace="openTrace" />
    </div>
</template>

<script>
import Led from '@/components/Led.vue';
import Heatmap from '@/components/Heatmap.vue';
import Chart from '@/components/Chart.vue';
import TracingTrace from '@/components/TracingTrace.vue';
import RumIntegration from '@/views/RumIntegration.vue';
import RumExplorer from '@/components/RumExplorer.vue';
import RumSessionPanel from '@/components/RumSessionPanel.vue';

const FILTER_KEYS = ['page', 'browser', 'os', 'device', 'country', 'version', 'percentile'];
const MORE_FILTER_KEYS = ['os', 'device', 'country', 'version'];

function queryWithoutExplorerLocal(q) {
    const out = { ...(q || {}) };
    delete out.rum_session;
    delete out.rum_q;
    return out;
}

function queryEqual(a, b) {
    const ak = Object.keys(a || {}).sort();
    const bk = Object.keys(b || {}).sort();
    if (ak.length !== bk.length) return false;
    for (let i = 0; i < ak.length; i++) {
        if (ak[i] !== bk[i]) return false;
        if (String(a[ak[i]] ?? '') !== String(b[bk[i]] ?? '')) return false;
    }
    return true;
}

const RumCwvBar = {
    name: 'RumCwvBar',
    props: { kpi: Object, formatCount: Function },
    template: `
        <v-tooltip v-if="kpi.bar" bottom>
            <template #activator="{ on }">
                <div v-on="on" class="mt-1">
                    <div class="d-flex rum-cwv-bar">
                        <div v-if="kpi.bar.good > 0" class="rum-cwv-seg" :style="{ width: kpi.bar.good + '%', background: '#4caf50', minWidth: '2px' }" />
                        <div v-if="kpi.bar.needs > 0" class="rum-cwv-seg" :style="{ width: kpi.bar.needs + '%', background: '#ff9800', minWidth: '2px' }" />
                        <div v-if="kpi.bar.poor > 0" class="rum-cwv-seg" :style="{ width: kpi.bar.poor + '%', background: '#f44336', minWidth: '2px' }" />
                    </div>
                    <div class="caption grey--text mt-1">{{ kpi.bar.good }}% good</div>
                </div>
            </template>
            <div class="pa-1">
                <div class="mb-1 font-weight-medium">{{ kpi.label }} rating</div>
                <div class="d-flex align-center"><span class="rum-dot" style="background:#4caf50" />Good {{ formatCount(kpi.counts.good) }} · {{ kpi.bar.good }}%</div>
                <div class="d-flex align-center"><span class="rum-dot" style="background:#ff9800" />Needs improvement {{ formatCount(kpi.counts.needs) }} · {{ kpi.bar.needs }}%</div>
                <div class="d-flex align-center"><span class="rum-dot" style="background:#f44336" />Poor {{ formatCount(kpi.counts.poor) }} · {{ kpi.bar.poor }}%</div>
                <div class="caption mt-1">{{ formatCount(kpi.counts.total) }} samples</div>
                <div v-if="kpi.bandHint" class="caption grey--text mt-1">{{ kpi.bandHint }}</div>
            </div>
        </v-tooltip>
    `,
};

export default {
    props: {
        appId: String,
    },
    components: { Led, Heatmap, Chart, TracingTrace, RumIntegration, RumExplorer, RumSessionPanel, RumCwvBar },
    data() {
        return {
            loading: false,
            explorerLoading: false,
            view: {},
            service: '',
            breakdownTab: 'browser',
            pageSort: { key: 'latency_ms', dir: -1 },
            contentWidth: 0,
            percentileItems: [
                { value: 50, text: 'p50' },
                { value: 75, text: 'p75' },
                { value: 90, text: 'p90' },
                { value: 95, text: 'p95' },
                { value: 99, text: 'p99' },
            ],
        };
    },
    computed: {
        rumQuery() {
            return this.$route.query.rum_q || '';
        },
        layout() {
            const w = this.contentWidth;
            if (w && w < 1100) return 'compact';
            if (w && w >= 1700) return 'wide';
            return 'regular';
        },
        filterBarTop() {
            const top = this.$vuetify && this.$vuetify.application && this.$vuetify.application.top;
            return typeof top === 'number' && top > 0 ? top : 64;
        },
        isRumClient() {
            return String(this.appId || '').includes(':RumClient:');
        },
        showStatusCard() {
            if (!this.isRumClient) return true;
            if (this.loading && !this.view.status) return false;
            return this.view.status && this.view.status !== 'ok';
        },
        services() {
            return (this.view.services || []).map((s) => ({ value: s.name, text: s.name + (s.linked ? ' (linked)' : '') }));
        },
        traceId() {
            return this.$route.query.rum_trace || '';
        },
        sessionId() {
            return this.$route.query.rum_session || '';
        },
        percentile() {
            const p = Number(this.$route.query.percentile || this.view.percentile || 75);
            return [50, 75, 90, 95, 99].includes(p) ? p : 75;
        },
        pctLabel() {
            return 'p' + this.percentile;
        },
        filters() {
            return {
                page: this.$route.query.page || '',
                browser: this.$route.query.browser || '',
                os: this.$route.query.os || '',
                device: this.$route.query.device || '',
                country: this.$route.query.country || '',
                version: this.$route.query.version || '',
            };
        },
        hasFilters() {
            return FILTER_KEYS.some((k) => k !== 'percentile' && this.$route.query[k]);
        },
        moreFiltersCount() {
            return MORE_FILTER_KEYS.filter((k) => this.$route.query[k]).length;
        },
        hasHeatmapSelection() {
            return !!this.$route.query.rum_sel;
        },
        heatmapSelection() {
            const raw = this.$route.query.rum_sel || '';
            if (!raw) return { x1: 0, x2: 0, y1: '', y2: '' };
            const parts = String(raw).split(':');
            let tsRange = '-',
                durRange = '-';
            if (parts.length >= 4) {
                tsRange = parts[2] || '-';
                durRange = parts[3] || '-';
            }
            const tp = tsRange.split('-');
            const dp = durRange.split('-');
            return {
                x1: Number(tp[0]) || 0,
                x2: Number(tp[1]) || 0,
                y1: dp[0] || '',
                y2: dp[1] || '',
            };
        },
        rateCharts() {
            return (this.view.charts || []).slice(0, 2);
        },
        cwvCharts() {
            return (this.view.charts || []).slice(2);
        },
        pageFilterItems() {
            return (this.view.top_pages || []).map((p) => p.path);
        },
        versionItems() {
            return (this.view.versions || []).map((v) => v.version);
        },
        summaryCards() {
            const s = this.view.summary || {};
            return [
                {
                    key: 'page_views',
                    label: 'Page views',
                    value: this.formatCount(s.page_views),
                    delta: s.delta_page_views,
                    hint: 'Number of document loads and SPA navigations in the selected time range. Delta is vs the previous equal-length window.',
                },
                {
                    key: 'sessions',
                    label: 'Sessions',
                    value: this.formatCount(s.unique_sessions),
                    hint: 'Unique browser session IDs observed in the selected time range.',
                },
                {
                    key: 'errors',
                    label: 'Browser errors',
                    value: this.formatCount(s.error_count),
                    sub: 'Fetch ' + this.formatPct(s.fetch_error_pct),
                    hint: 'JavaScript errors (window.onerror / unhandledrejection). Fetch % is failed XHR/fetch requests over total browser HTTP calls.',
                },
            ];
        },
        cwvKpis() {
            const s = this.view.summary || {};
            const th = this.view.thresholds || {};
            const hints = {
                lcp: 'Largest Contentful Paint — time until the largest content element is rendered.',
                inp: 'Interaction to Next Paint — responsiveness of user interactions.',
                cls: 'Cumulative Layout Shift — visual stability (unexpected layout shifts).',
                ttfb: 'Time to First Byte — time until the browser receives the first byte of the response.',
            };
            const mk = (key, label, value, delta, good, needs, poor, threshold, cls) => {
                const tone = this.percentile === 75 ? this.vitalTone(value, threshold, cls) : '';
                const status = this.percentile === 75 ? this.vitalStatus(value, threshold, cls) : '';
                const g = good || 0;
                const n = needs || 0;
                const p = poor || 0;
                const total = g + n + p;
                let bar = null;
                if (total > 0) {
                    bar = {
                        good: Math.round((g / total) * 100),
                        needs: Math.round((n / total) * 100),
                        poor: Math.round((p / total) * 100),
                    };
                }
                return {
                    key,
                    label,
                    display: cls ? this.formatCls(value) : this.formatMs(value),
                    delta,
                    cls,
                    tone,
                    status,
                    bar,
                    counts: { good: g, needs: n, poor: p, total },
                    bandHint: this.vitalBandHint(threshold, cls),
                    hint: hints[key] || '',
                };
            };
            return [
                mk('lcp', 'LCP', s.lcp_ms, s.delta_lcp_ms, s.good_lcp, s.needs_lcp, s.poor_lcp, th.lcp_ms),
                mk('inp', 'INP', s.inp_ms, s.delta_inp_ms, s.good_inp, s.needs_inp, s.poor_inp, th.inp_ms),
                mk('cls', 'CLS', s.cls, s.delta_cls, s.good_cls, s.needs_cls, s.poor_cls, th.cls, true),
                mk('ttfb', 'TTFB', s.ttfb_ms, s.delta_ttfb_ms, s.good_ttfb, s.needs_ttfb, s.poor_ttfb, th.ttfb_ms),
            ];
        },
        sortedPages() {
            const rows = [...(this.view.top_pages || [])];
            const { key, dir } = this.pageSort;
            rows.sort((a, b) => {
                const av = a[key] == null ? 0 : a[key];
                const bv = b[key] == null ? 0 : b[key];
                if (av < bv) return -1 * dir;
                if (av > bv) return 1 * dir;
                return 0;
            });
            return rows;
        },
        breakdownLabel() {
            return { browser: 'Browser', os: 'OS', device: 'Device', country: 'Country', version: 'Version' }[this.breakdownTab] || 'Name';
        },
        breakdownFilterKey() {
            return this.breakdownTab === 'version' ? 'version' : this.breakdownTab;
        },
        breakdownFilterValue() {
            return this.filters[this.breakdownFilterKey] || '';
        },
        breakdownRows() {
            switch (this.breakdownTab) {
                case 'os':
                    return this.view.operating_systems || [];
                case 'device':
                    return this.view.devices || [];
                case 'country':
                    return this.view.countries || [];
                case 'version':
                    return (this.view.versions || []).map((v) => ({
                        name: v.version,
                        count: v.count,
                        latency_ms: v.latency_ms,
                        error_count: v.error_count,
                    }));
                default:
                    return this.view.browsers || [];
            }
        },
    },
    watch: {
        appId: 'get',
        '$route.query': {
            handler(next, prev) {
                // Session panel / explorer query load their own data; don't reload the whole dashboard.
                if (queryEqual(queryWithoutExplorerLocal(next), queryWithoutExplorerLocal(prev))) {
                    const qChanged = String((next && next.rum_q) || '') !== String((prev && prev.rum_q) || '');
                    if (qChanged) this.getExplorer();
                    return;
                }
                this.get();
            },
            deep: true,
        },
    },
    mounted() {
        this.contentWidth = this.$el ? this.$el.clientWidth : 0;
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },
    methods: {
        onContentResize(entries) {
            const entry = entries && entries[0];
            if (entry && entry.contentRect) {
                this.contentWidth = entry.contentRect.width;
            } else if (this.$el) {
                this.contentWidth = this.$el.clientWidth;
            }
        },
        nameItems(list) {
            return (list || []).map((x) => x.name);
        },
        formatCount(v) {
            if (v == null || v === 0) return '0';
            return Number(v).toLocaleString();
        },
        formatMs(v) {
            if (v == null || v === 0 || Number.isNaN(Number(v))) return '—';
            return Math.round(Number(v)) + ' ms';
        },
        formatPct(v) {
            if (v == null || Number.isNaN(Number(v))) return '—';
            return Number(v).toFixed(1) + '%';
        },
        formatCls(v) {
            if (v == null || Number.isNaN(Number(v))) return '—';
            if (v === 0) return '0';
            return Number(v).toFixed(3);
        },
        formatDelta(v, cls) {
            if (v == null || v === 0) return '';
            const sign = v > 0 ? '+' : '';
            if (cls) return sign + Number(v).toFixed(3);
            if (Math.abs(v) >= 1) return sign + Math.round(v);
            return sign + Number(v).toFixed(1);
        },
        deltaClass(v) {
            if (v == null || v === 0) return 'grey--text';
            return v > 0 ? 'error--text' : 'success--text';
        },
        vitalPoorCutoff(threshold, cls) {
            if (threshold == null) return null;
            if (cls) return 0.25;
            if (threshold <= 200) return 500; // INP/FID
            if (threshold <= 800) return 1800; // TTFB
            if (threshold <= 2500) return 4000; // LCP/FCP
            return threshold * 1.6;
        },
        vitalBandHint(threshold, cls) {
            if (threshold == null) return '';
            const poor = this.vitalPoorCutoff(threshold, cls);
            if (cls) return `good ≤ ${threshold}, poor > ${poor}`;
            return `good ≤ ${Math.round(threshold)} ms, poor > ${Math.round(poor)} ms`;
        },
        vitalBand(value, threshold, cls) {
            if (value == null || threshold == null) return '';
            const poorCutoff = this.vitalPoorCutoff(threshold, cls);
            if (value <= threshold) return 'ok';
            if (value <= poorCutoff) return 'warning';
            return 'critical';
        },
        vitalTone(value, threshold, cls) {
            const s = this.vitalBand(value, threshold, cls);
            if (s === 'ok') return 'success--text';
            if (s === 'warning') return 'warning--text';
            if (s === 'critical') return 'error--text';
            return '';
        },
        vitalStatus(value, threshold, cls) {
            return this.vitalBand(value, threshold, cls);
        },
        filterParams() {
            const params = {};
            FILTER_KEYS.forEach((k) => {
                const v = this.$route.query[k];
                if (v) params[k] = v;
            });
            if (!params.percentile) params.percentile = String(this.percentile);
            if (this.$route.query.rum_sel) params.rum_sel = this.$route.query.rum_sel;
            if (this.rumQuery) params.rum_q = this.rumQuery;
            return params;
        },
        get() {
            this.loading = true;
            const params = this.filterParams();
            if (this.traceId) {
                params.trace = this.traceId;
            }
            this.$api.getRum(this.appId, params, (data, error) => {
                this.loading = false;
                if (error) {
                    this.view = { status: 'warning', message: error };
                    return;
                }
                this.view = data || {};
                const linked = (this.view.services || []).find((s) => s.linked);
                this.service = linked ? linked.name : '';
            });
        },
        getExplorer() {
            this.explorerLoading = true;
            const params = { ...this.filterParams(), only: 'explorer' };
            this.$api.getRum(this.appId, params, (data, error) => {
                this.explorerLoading = false;
                if (error) {
                    this.$set(this.view, 'explorer_error', error);
                    return;
                }
                const d = data || {};
                if (d.explorer_error) {
                    this.$set(this.view, 'explorer_error', d.explorer_error);
                    this.$set(this.view, 'explorer_query', d.explorer_query || this.rumQuery);
                    return;
                }
                this.$set(this.view, 'explorer_error', '');
                this.$set(this.view, 'explorer_query', d.explorer_query || this.rumQuery);
                this.$set(this.view, 'sessions', d.sessions || []);
                this.$set(this.view, 'page_views', d.page_views || []);
                this.$set(this.view, 'sessions_limited', !!d.sessions_limited);
                this.$set(this.view, 'page_views_limited', !!d.page_views_limited);
            });
        },
        setRumQuery(q) {
            const next = String(q || '').trim();
            const cur = String(this.rumQuery || '').trim();
            if (next === cur) return;
            this.setFilter('rum_q', next);
        },
        changeService(name) {
            this.$api.saveRumSettings(this.appId, { service: name }, () => this.get());
        },
        setFilter(key, value) {
            const q = { ...this.$route.query };
            if (value == null || value === '') delete q[key];
            else q[key] = String(value);
            this.$router.replace({ query: q }).catch(() => {});
        },
        toggleFilter(key, value) {
            if (this.filters[key] === value || this.$route.query[key] === String(value)) {
                this.setFilter(key, '');
            } else {
                this.setFilter(key, value);
            }
        },
        clearFilters() {
            const q = { ...this.$route.query };
            FILTER_KEYS.forEach((k) => {
                if (k !== 'percentile') delete q[k];
            });
            delete q.rum_sel;
            this.$router.replace({ query: q }).catch(() => {});
        },
        applyBreakdownFilter(name) {
            this.toggleFilter(this.breakdownFilterKey, name);
        },
        clearBreakdownFilter() {
            this.setFilter(this.breakdownFilterKey, '');
        },
        sortBy(key) {
            if (this.pageSort.key === key) this.pageSort.dir *= -1;
            else this.pageSort = { key, dir: -1 };
        },
        setHeatmapSelection(s) {
            const hm = this.view.heatmap;
            if (!hm || !hm.series) return;
            if (s === 'errors') s = { y1: 'inf', y2: 'err' };
            const tsRange = `${s.x1 || ''}-${s.x2 || ''}`;
            const durRange = `${s.y1 || ''}-${s.y2 || ''}`;
            const rum_sel = `::${tsRange}:${durRange}:`;
            const { from, to } = hm.ctx;
            const query = { ...this.$route.query, rum_sel, from, to };
            delete query.rum_trace;
            this.$router.push({ query }).catch(() => {});
        },
        openTrace(id) {
            const q = { ...this.$route.query };
            delete q.rum_session;
            if (id) {
                q.rum_trace = id;
                this.$router.push({ query: q }).catch(() => {});
                return;
            }
            delete q.rum_trace;
            this.$router.replace({ query: q }).catch(() => {});
        },
        openSession(id) {
            const q = { ...this.$route.query };
            if (id) {
                delete q.rum_trace;
                q.rum_session = id;
            } else {
                delete q.rum_session;
            }
            this.$router.replace({ query: q }).catch(() => {});
        },
    },
};
</script>

<style scoped>
.rum-filters {
    background: var(--v-background-base, #fff);
}
.rum-kpi-info {
    opacity: 0.55;
    cursor: help;
}
.rum-kpi-info:hover {
    opacity: 1;
}
.rum-kpi-wide {
    flex-wrap: nowrap;
}
.rum-kpi-wide >>> .rum-kpi-flex {
    flex: 1 1 0;
    max-width: none;
    width: 0;
    padding: 4px;
}
.rum-row-selected,
.rum-row-selected > td {
    background-color: rgba(3, 169, 244, 0.22) !important;
}
.rum-cell-truncate {
    max-width: 280px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.rum-cwv-bar {
    height: 8px;
    border-radius: 2px;
    overflow: hidden;
}
.rum-cwv-seg {
    height: 100%;
}
.rum-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    margin-right: 6px;
    flex-shrink: 0;
}
</style>

<style>
/* Shared with inline RumCwvBar tooltip content */
.rum-cwv-bar {
    height: 8px;
    border-radius: 2px;
    overflow: hidden;
}
.rum-cwv-seg {
    height: 100%;
}
.rum-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    margin-right: 6px;
    flex-shrink: 0;
}
</style>
