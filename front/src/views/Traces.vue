<template>
    <div class="traces">
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <v-alert v-else-if="view.message" color="info" outlined text class="message">
            <template v-if="view.message === 'not_found'">
                This page only shows traces from OpenTelemetry integrations, not from eBPF.
                <div class="mt-2">
                    <OpenTelemetryIntegration color="primary">Integrate OpenTelemetry</OpenTelemetryIntegration>
                </div></template
            >
            <template v-if="view.message === 'no_clickhouse'"> Clickhouse integration is not configured. </template>
        </v-alert>

        <template v-else>
            <div class="mt-4 d-flex">
                <v-spacer />
                <OpenTelemetryIntegration small color="primary">Integrate OpenTelemetry</OpenTelemetryIntegration>
            </div>

            <v-alert v-if="view.error" color="error" icon="mdi-alert-octagon-outline" outlined text class="mt-2">
                {{ view.error }}
            </v-alert>

            <Heatmap v-if="view.heatmap" :heatmap="view.heatmap" :selection="selection" @select="setSelection" :loading="loading" />

            <v-tabs height="32" show-arrows hide-slider>
                <v-tab v-for="v in views" :key="v.name" :to="openView(v.name)" class="view" :class="{ active: query.view === v.name }">
                    <v-icon small class="mr-1">{{ v.icon }}</v-icon>
                    {{ v.title }}
                </v-tab>
            </v-tabs>

            <v-card outlined class="query px-4 py-2 my-4">
                <div class="mt-2 d-flex align-center" style="gap: 4px">
                    <div>Filters:</div>
                    <div class="d-flex flex-wrap align-center filters">
                        <div v-for="(f, i) in filters" class="d-flex align-center filter">
                            <template v-if="f.edit">
                                <v-select
                                    v-model="f.field"
                                    :items="Object.keys(filterable.fields).map((f) => ({ value: f, text: filterable.fields[f] }))"
                                    outlined
                                    dense
                                    hide-details
                                    :menu-props="{ 'offset-y': true }"
                                    append-icon="mdi-chevron-down"
                                    class="field"
                                />
                                <v-select
                                    v-model="f.op"
                                    :items="filterable.ops"
                                    outlined
                                    dense
                                    hide-details
                                    :menu-props="{ 'offset-y': true }"
                                    append-icon="mdi-chevron-down"
                                    class="op"
                                />
                                <v-text-field outlined dense v-model="f.value" hide-details class="value" />
                                <v-btn @click="applyFilters" :disabled="!f.field" small icon>
                                    <v-icon small color="success">mdi-check</v-icon>
                                </v-btn>
                                <v-btn @click="delFilter(i)" small icon>
                                    <v-icon small color="error">mdi-close</v-icon>
                                </v-btn>
                            </template>
                            <template v-else>
                                <v-chip @click="editFilter(i)" @click:close="delFilter(i)" label close color="primary">
                                    <div class="where-arg">{{ filterable.fields[f.field] }} {{ f.op }} {{ f.value }}</div>
                                </v-chip>
                            </template>
                        </div>
                        <v-btn v-if="!filters.some((f) => f.edit)" @click="newFilter" small icon>
                            <v-icon small>mdi-plus</v-icon>
                        </v-btn>
                    </div>
                </div>
                <div class="d-flex align-center">
                    <div><div class="marker selection"></div></div>
                    <div>
                        Selection:
                        <template v-if="selectionDefined">
                            <template v-if="query.ts_from && query.ts_to">
                                time <var> {{ format(query.ts_from, 'ts') }}</var> — <var> {{ format(query.ts_to, 'ts') }}</var>
                            </template>
                            <template v-if="query.dur_from !== 'inf' || query.dur_to === 'err'">
                                where (
                                <template v-if="query.dur_from !== 'inf'">
                                    trace duration
                                    <var> {{ format(query.dur_from, 'dur') }}</var> — <var> {{ format(query.dur_to, 'dur') }}</var>
                                    <template v-if="query.dur_to === 'err'"> or </template>
                                </template>
                                <template v-if="query.dur_to === 'err'"> trace status is <var> Error</var></template>
                                )
                            </template>
                            <v-tooltip bottom>
                                <template #activator="{ on }">
                                    <v-btn :to="clearSelection()" v-on="on" x-small icon exact><v-icon small>mdi-close</v-icon></v-btn>
                                </template>
                                <v-card class="px-2 py-1"> clear selection </v-card>
                            </v-tooltip>
                        </template>
                        <span v-else class="grey--text">
                            <template v-if="query.view === 'attributes'">select a chart area to explore trace attributes</template>
                            <template v-else>select a chart area to see traces for a specific time range, duration, or status</template>
                        </span>
                    </div>
                </div>
                <div v-if="query.view === 'attributes' || query.view === 'latency'" class="d-flex align-center">
                    <div><div class="marker baseline"></div></div>
                    Baseline: other events within the time window
                </div>
                <v-form :disabled="loading">
                    <v-checkbox
                        v-model="form.excludeAux"
                        label="Exclude auxiliary requests (from monitoring, control plane, etc)"
                        dense
                        hide-details
                    />
                    <div v-if="query.view === 'latency'" class="d-flex mt-2 mb-1 align-baseline" style="gap: 8px; min-width: 0">
                        <div>View:</div>
                        <v-btn-toggle :value="query.diff || false" @change="setDiffMode" mandatory>
                            <v-btn :value="false" height="30">
                                <v-icon small class="mr-1">mdi-chart-timeline</v-icon>
                                FlameGraph
                            </v-btn>
                            <v-btn :value="true" height="30" :disabled="!selectionDefined">
                                <v-icon small class="mr-1 mdi-flip-h">mdi-select-compare</v-icon>
                                Diff
                            </v-btn>
                        </v-btn-toggle>
                    </div>
                </v-form>
                <v-progress-linear v-if="loading" indeterminate height="4" style="position: absolute; bottom: 0; left: 0" />
            </v-card>

            <div v-if="query.trace_id" class="mt-5" style="min-height: 50vh">
                <div class="text-md-h6 mb-3">
                    <router-link :to="openView('traces')">
                        <v-icon>mdi-arrow-left</v-icon>
                    </router-link>
                    Trace {{ query.trace_id }}
                </div>
                <TracingTrace v-if="view.trace" :spans="view.trace" />
            </div>

            <div v-else-if="query.view === 'overview'" class="mt-5" style="min-height: 50vh">
                <v-data-table
                    :items="view.summary ? view.summary.stats : []"
                    :items-per-page="50"
                    sort-by="total"
                    sort-desc
                    must-sort
                    dense
                    class="table"
                    mobile-breakpoint="0"
                    no-data-text="No traces found"
                    :headers="[
                        { value: 'service_name', text: 'Root Service Name', align: 'start' },
                        { value: 'span_name', text: 'Root Span Name', align: 'start' },
                        { value: 'total', text: 'Requests', align: 'end' },
                        { value: 'failed', text: 'Errors', align: 'end' },
                        { value: 'duration_quantiles[0]', text: 'p50', align: 'end' },
                        { value: 'duration_quantiles[1]', text: 'p95', align: 'end' },
                        { value: 'duration_quantiles[2]', text: 'p99', align: 'end' },
                    ]"
                    :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
                >
                    <template #item.service_name="{ item }">
                        <router-link :to="filterTraces(item.service_name)">
                            {{ item.service_name }}
                        </router-link>
                    </template>
                    <template #item.span_name="{ item }">
                        <router-link :to="filterTraces(item.service_name, item.span_name)">
                            {{ item.span_name }}
                        </router-link>
                    </template>
                    <template #item.total="{ item }">
                        <span>{{ format(item.total) }}</span>
                        <span class="caption grey--text">/s</span>
                    </template>
                    <template #item.failed="{ item }">
                        <router-link v-if="item.failed" :to="filterTraces(item.service_name, item.span_name, true)">
                            <span>{{ format(item.failed, '%') }}</span>
                            <span class="caption grey--text">%</span>
                        </router-link>
                        <span v-else>—</span>
                    </template>
                    <template #item.duration_quantiles[0]="{ item }">
                        <span>{{ format(item.duration_quantiles[0], 'ms') }}</span>
                        <span class="caption grey--text"> ms</span>
                    </template>
                    <template #item.duration_quantiles[1]="{ item }">
                        <span>{{ format(item.duration_quantiles[1], 'ms') }}</span>
                        <span class="caption grey--text"> ms</span>
                    </template>
                    <template #item.duration_quantiles[2]="{ item }">
                        <span>{{ format(item.duration_quantiles[2], 'ms') }}</span>
                        <span class="caption grey--text"> ms</span>
                    </template>

                    <template #foot>
                        <tfoot>
                            <tr v-for="item in view.summary ? [view.summary.overall] : []">
                                <td class="font-weight-medium">OVERALL</td>
                                <td></td>
                                <td class="text-right font-weight-medium">
                                    <span>{{ format(item.total) }}</span>
                                    <span class="caption grey--text">/s</span>
                                </td>
                                <td class="text-right font-weight-medium">
                                    <router-link v-if="item.failed" :to="filterTraces(query.service_name, query.span_name, true)">
                                        <span>{{ format(item.failed, '%') }}</span>
                                        <span class="caption grey--text">%</span>
                                    </router-link>
                                    <span v-else>—</span>
                                </td>
                                <td class="text-right font-weight-medium">
                                    <span>{{ format(item.duration_quantiles[0], 'ms') }}</span>
                                    <span class="caption grey--text"> ms</span>
                                </td>
                                <td class="text-right font-weight-medium">
                                    <span>{{ format(item.duration_quantiles[1], 'ms') }}</span>
                                    <span class="caption grey--text"> ms</span>
                                </td>
                                <td class="text-right font-weight-medium">
                                    <span>{{ format(item.duration_quantiles[2], 'ms') }}</span>
                                    <span class="caption grey--text"> ms</span>
                                </td>
                            </tr>
                        </tfoot>
                    </template>
                </v-data-table>
            </div>

            <div v-else-if="query.view === 'traces'" class="mt-5" style="min-height: 50vh">
                <v-simple-table dense>
                    <thead>
                        <tr>
                            <th>Trace ID</th>
                            <th>Root Service</th>
                            <th>Name</th>
                            <th>Status</th>
                            <th>Duration</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="s in view.traces">
                            <td>
                                <router-link :to="openTrace(s.trace_id)" exact class="text-no-wrap">
                                    <v-icon small style="vertical-align: baseline">mdi-chart-timeline</v-icon>
                                    {{ s.trace_id.substring(0, 8) }}
                                </router-link>
                            </td>
                            <td class="text-no-wrap">{{ s.service }}</td>
                            <td class="text-no-wrap">{{ s.name }}</td>
                            <td class="text-no-wrap">
                                <v-icon v-if="s.status.error" color="error" small class="ml-1" style="margin-bottom: 2px">mdi-alert-circle</v-icon>
                                <v-icon v-else color="success" small class="ml-1" style="margin-bottom: 2px">mdi-check-circle</v-icon>
                                {{ s.status.message }}
                            </td>
                            <td class="text-no-wrap">
                                {{ format(s.duration, 'ms') }}
                                <span class="caption grey--text"> ms</span>
                            </td>
                        </tr>
                    </tbody>
                </v-simple-table>
                <div v-if="!loading && (!view.traces || !view.traces.length)" class="pa-3 text-center grey--text">No traces found</div>
                <div v-if="!loading && view.traces && view.traces.length && view.limit" class="text-right caption grey--text">
                    The output is capped at {{ view.limit }} traces.
                </div>
            </div>

            <div v-else-if="query.view === 'attributes'">
                <div class="d-flex grey--text mt-2 mb-3">
                    <v-icon small class="mr-1">mdi-information-outline</v-icon>
                    This section shows how the attributes of traces in the selected area differ from those of other traces.
                </div>
                <div class="attr-stats" :style="{ gap: statsStyles.gap }">
                    <div v-for="attr in view.attr_stats" class="attr" :style="{ width: statsStyles.attrWidth }">
                        <div class="name">{{ attr.name }}</div>
                        <v-tooltip v-for="v in attr.values" bottom transition="none" attach=".attr-stats" content-class="attr-value-details">
                            <template #activator="{ on }">
                                <router-link :to="openTrace(v.sample_trace_id)">
                                    <div class="value" v-on="on">
                                        <div class="name">
                                            {{ v.name }}
                                        </div>
                                        <div class="bars">
                                            <div class="bar baseline" :style="{ width: v.baseline * 100 + '%' }"></div>
                                            <div class="bar selection" :style="{ width: v.selection * 100 + '%' }"></div>
                                        </div>
                                    </div>
                                </router-link>
                            </template>
                            <v-card class="pa-2">
                                <div>Value:</div>
                                <div class="font-weight-medium mb-1">{{ v.name }}</div>
                                <div class="baseline">
                                    <span class="marker" />
                                    Baseline: {{ v.baseline ? format(v.baseline, '%') + '%' : '—' }}
                                </div>
                                <div class="selection">
                                    <span class="marker" />
                                    Selection: {{ v.selection ? format(v.selection, '%') + '%' : '—' }}
                                </div>
                                <div class="d-flex grey--text mt-2">
                                    <v-icon x-small class="mr-1">mdi-information-outline</v-icon>
                                    Click to view a sample trace containing this attribute
                                </div>
                            </v-card>
                        </v-tooltip>
                    </div>
                </div>
            </div>

            <div v-else-if="query.view === 'errors'">
                <div class="d-flex grey--text mt-2 mb-3">
                    <v-icon small class="mr-2">mdi-information-outline</v-icon>
                    This section highlights the underlying reasons why traces within the selected range contain errors. It identifies the tracing
                    spans where errors originated.
                </div>
                <v-data-table
                    :items="view.errors || []"
                    :items-per-page="20"
                    sort-by="count"
                    sort-desc
                    must-sort
                    class="table errors"
                    mobile-breakpoint="0"
                    no-data-text="No errors found"
                    :headers="[
                        { value: 'service_name', text: 'Service Name', width: '15%' },
                        { value: 'span_name', text: 'Span', width: '25%' },
                        { value: 'sample_error', text: 'Error', width: '40%' },
                        { value: 'sample_trace_id', text: 'Sample Trace', sortable: false, width: '16ch' },
                        { value: 'count', text: 'Percentage', width: '16ch' },
                    ]"
                    :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
                >
                    <template #item.service_name="{ item }">
                        <span :title="item.service_name" class="service nowrap" :style="{ borderColor: color(item.service_name) }">
                            {{ item.service_name }}
                        </span>
                    </template>
                    <template #item.span_name="{ item }">
                        <div class="nowrap" :title="item.span_name">{{ item.span_name }}</div>
                        <div v-for="(v, k) in item.labels" :title="`${k}: ${v}`" class="caption nowrap" style="line-height: 1rem">
                            • {{ k }}: {{ v }}
                        </div>
                    </template>
                    <template #item.sample_error="{ item }">
                        <div v-if="item.sample_error" class="nowrap" :title="item.sample_error">
                            <v-icon color="error" small style="margin-bottom: 2px">mdi-alert-circle</v-icon>
                            {{ item.sample_error }}
                        </div>
                    </template>
                    <template #item.sample_trace_id="{ item }">
                        <router-link :to="openTrace(item.sample_trace_id)" exact class="nowrap">
                            <v-icon small style="vertical-align: baseline">mdi-chart-timeline</v-icon>
                            {{ item.sample_trace_id.substring(0, 8) }}
                        </router-link>
                    </template>
                    <template #item.count="{ item }">
                        <div class="d-flex align-center" style="gap: 4px">
                            <div style="text-align: right; width: 4ch">
                                <span>{{ format(item.count, '%') }}</span>
                                <span class="caption grey--text">%</span>
                            </div>
                            <div class="flex-grow-1">
                                <v-progress-linear :value="item.count * 100" background-opacity="0" height="14" />
                            </div>
                        </div>
                    </template>
                </v-data-table>
            </div>

            <div v-else-if="query.view === 'latency'">
                <div class="d-flex grey--text mt-2 mb-3">
                    <v-icon small class="mr-1">mdi-information-outline</v-icon>
                    This section shows the latency FlameGraph for the selected traces. A wider frame indicates greater time consumption by that
                    tracing span.
                </div>
                <FlameGraph
                    v-if="view.latency"
                    :profile="view.latency"
                    :actions="[{ title: 'Open a sample trace', icon: 'mdi-chart-timeline', to: (s) => openTrace(s.data['trace_id']) }]"
                    class="pt-2"
                />
            </div>
        </template>
    </div>
</template>

<script>
import { palette } from '../utils/colors';
import Heatmap from '../components/Heatmap.vue';
import TracingTrace from '../components/TracingTrace.vue';
import FlameGraph from '../components/FlameGraph.vue';
import OpenTelemetryIntegration from '@/views/OpenTelemetryIntegration.vue';

export default {
    components: { OpenTelemetryIntegration, FlameGraph, TracingTrace, Heatmap },

    data() {
        return {
            view: {},
            filters: [],
            form: {
                excludeAux: true,
                diff: false,
            },
            loading: false,
            error: '',
        };
    },

    mounted() {
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        form: {
            handler(v) {
                this.setForm(v);
            },
            deep: true,
        },
        '$route.query.query': {
            handler() {
                this.filters = (this.query.filters || []).map((f) => ({ ...f, edit: false }));
                this.form.excludeAux = !this.query.include_aux;
                this.form.diff = (this.selectionDefined ? this.query.diff : false) || false;
                this.get();
            },
            immediate: true,
        },
    },

    computed: {
        views() {
            return [
                { name: 'overview', title: 'overview', icon: 'mdi-format-list-checkbox' },
                { name: 'traces', title: 'traces', icon: 'mdi-chart-timeline' },
                { name: 'errors', title: 'error causes', icon: 'mdi-target' },
                { name: 'latency', title: 'latency explorer', icon: 'mdi-clock-fast' },
                { name: 'attributes', title: 'compare attributes', icon: 'mdi-select-compare' },
            ];
        },
        query() {
            let q = {};
            try {
                q = JSON.parse(this.$route.query.query);
            } catch {
                //
            }
            if (!q.view) {
                q.view = 'overview';
            }
            return q;
        },
        filterable() {
            return {
                fields: {
                    ServiceName: 'Root Service Name',
                    SpanName: 'Root Span Name',
                    TraceId: 'Trace ID',
                },
                ops: ['=', '!=', '~', '!~'],
            };
        },
        selection() {
            const q = this.query;
            return { x1: q.ts_from, x2: q.ts_to, y1: q.dur_from, y2: q.dur_to };
        },
        selectionDefined() {
            const q = this.query;
            return q.ts_from || q.ts_to || q.dur_from || q.dur_to;
        },
        statsStyles() {
            const gap = 8;
            const cols = { xs: 2, sm: 3, md: 4, lg: 5, xl: 6 }[this.$vuetify.breakpoint.name];
            return {
                gap: gap + 'px',
                attrWidth: `calc((100% - ${(cols - 1) * gap}px) / ${cols})`,
            };
        },
    },

    methods: {
        get() {
            const query = this.$route.query.query || '';
            this.loading = true;
            this.error = '';
            this.$api.getOverview('traces', query, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.view = data.traces || {};
            });
        },
        push(to) {
            this.$router.push(to).catch((err) => err);
        },
        setQuery(q, from, to) {
            const query = q ? JSON.stringify(q) : undefined;
            return { query: { query, from, to } };
        },
        filterTraces(serviceName, spanName, errors) {
            const { from, to } = this.$route.query;
            const q = { ...this.query };
            q.filters = [];
            if (serviceName) {
                q.filters.push({ field: 'ServiceName', op: '=', value: serviceName });
            }
            if (spanName) {
                q.filters.push({ field: 'SpanName', op: '=', value: spanName });
                q.view = 'traces';
            }
            if (errors) {
                q.view = 'errors';
                q.dur_from = 'inf';
                q.dur_to = 'err';
            }
            return this.setQuery(q, from, to);
        },
        openTrace(id) {
            const { from, to } = this.$route.query;
            const q = { ...this.query };
            q.view = 'traces';
            q.trace_id = id;
            return this.setQuery(q, from, to);
        },
        openView(v) {
            const { from, to } = this.$route.query;
            const q = { ...this.query };
            q.view = v;
            q.trace_id = undefined;
            if (!v || v === 'overview') {
                q.ts_from = undefined;
                q.ts_to = undefined;
                q.dur_from = undefined;
                q.dur_to = undefined;
            }
            return this.setQuery(q, from, to);
        },
        newFilter() {
            this.filters.push({ field: '', op: '=', value: '', edit: true });
        },
        delFilter(i) {
            this.filters.splice(i, 1);
            this.applyFilters();
        },
        editFilter(i) {
            this.filters[i].edit = true;
        },
        applyFilters() {
            this.filters.forEach((f) => {
                f.edit = !f.field;
            });
            const { from, to } = this.$route.query;
            const q = { ...this.query };
            q.filters = this.filters.filter((f) => !f.edit).map(({ field, op, value }) => ({ field, op, value }));
            if (!q.filters.length) {
                q.filters = undefined;
            }
            if (q.view === 'overview' && q.filters && q.filters.some((f) => f.field === 'TraceId')) {
                q.view = 'traces';
            }
            this.push(this.setQuery(q, from, to));
        },
        clearSelection() {
            const { from, to } = this.$route.query;
            const q = { ...this.query };
            q.ts_from = undefined;
            q.ts_to = undefined;
            q.dur_from = undefined;
            q.dur_to = undefined;
            q.diff = undefined;
            return this.setQuery(q, from, to);
        },
        setSelection(s) {
            const { from, to } = this.view.heatmap.ctx;
            const q = { ...this.query };
            q.ts_from = s.x1;
            q.ts_to = s.x2;
            q.dur_from = s.y1;
            q.dur_to = s.y2;
            q.trace_id = undefined;
            if (!q.view || q.view === 'overview') {
                q.view = 'traces';
            }
            this.push(this.setQuery(q, from, to));
        },
        setDiffMode(m) {
            const { from, to } = this.$route.query;
            const q = { ...this.query };
            q.diff = m || undefined;
            this.push(this.setQuery(q, from, to));
        },
        setForm(f) {
            const { from, to } = this.$route.query;
            const q = { ...this.query };
            q.include_aux = !f.excludeAux;
            q.diff = f.diff;
            this.push(this.setQuery(q, from, to));
        },
        color(s) {
            return palette.hash2(s);
        },
        format(v, unit) {
            if (unit === 'ts') {
                return this.$format.date(v, '{MMM} {DD}, {HH}:{mm}');
            }
            if (unit === 'dur') {
                if (!v) {
                    return '0';
                }
                if (v === 'inf' || v === 'err') {
                    return 'Inf';
                }
                if (v >= 1) {
                    return v + 's';
                }
                return v * 1000 + 'ms';
            }
            if (unit === '%') {
                v *= 100;
                if (v < 1) {
                    return '<1';
                }
                let d = 1;
                if (v >= 10) {
                    d = 0;
                }
                return v.toFixed(d);
            }
            if (unit === 'ms') {
                let d = 0;
                if (v < 10) {
                    d = 1;
                }
                return v.toFixed(d);
            }
            let m = '';
            if (v > 1e3) {
                v /= 1000;
                m = 'K';
            }
            if (v > 1e6) {
                v /= 1000;
                m = 'M';
            }
            if (v > 1e9) {
                v /= 1000;
                m = 'G';
            }
            return v.toFixed(1) + m;
        },
    },
};
</script>

<style scoped>
.traces {
    --baseline-color: #42a5f5;
    --selection-color: #ffca28;
}

.message a {
    font-weight: 500;
}

.view {
    color: var(--text-color-dimmed);
}
.view.active {
    color: var(--text-color);
    border-bottom: 2px solid var(--text-color);
}

.query {
    display: flex;
    flex-direction: column;
    gap: 8px;
}
.query var {
    font-style: normal;
    font-weight: 500;
}
.query .marker {
    height: 16px;
    width: 16px;
    margin-right: 4px;
}
.query .marker.baseline {
    background-color: var(--baseline-color);
}
.query .marker.selection {
    background-color: var(--selection-color);
}
.query .where-arg {
    max-width: 100%;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.query:deep(.v-chip) {
    height: 26px;
}
.query:deep(.v-input--checkbox) {
    margin-top: 0;
    margin-left: -4px;
    padding-top: 0;
}
.query:deep(.v-input--checkbox) label {
    margin-left: -8px;
    color: var(--text-color);
}

.filters {
    gap: 8px;
    min-width: 0;
}
.filter {
    gap: 4px;
    max-width: 100%;
}
.filter * {
    font-size: 14px;
}
.filter:deep(.v-input__slot) {
    min-height: initial !important;
    height: 26px !important;
    padding: 0 8px !important;
}
.filter:deep(.v-select__selection--comma) {
    margin: 0 !important;
}
.filter:deep(.v-input__append-inner) {
    margin-top: 2px !important;
    margin-right: -8px !important;
}
.filter:deep(.v-icon) {
    font-size: 16px;
}
*:deep(.v-list-item) {
    font-size: 14px;
    min-height: 32px !important;
    padding: 0 8px !important;
}
.filter .field {
    max-width: 22ch;
}
.filter .op {
    max-width: 8ch;
}

.attr-stats {
    display: flex;
    flex-wrap: wrap;
}
.attr-stats .attr {
    border: 1px solid var(--border-color);
    border-radius: 4px;
    font-size: 12px;
    padding: 8px 4px;
}
.attr-stats .attr .name {
    font-weight: 700;
    font-size: 14px;
    color: var(--text-color-dimmed);
    margin-bottom: 8px;
    padding: 0 4px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.attr-stats .attr .value {
    display: flex;
    align-items: center;
    cursor: pointer;
    margin-bottom: 4px;
    padding: 0 4px;
    gap: 4px;
}
.attr-stats .attr .value:hover {
    background-color: var(--background-color-hi);
}
.attr-stats .attr .value .name {
    font-weight: normal;
    color: var(--text-color);
    margin-bottom: 0;
    width: 60%;
    padding: 0;
}
.attr-stats .attr .value .bars {
    width: 40%;
}
.attr-stats .attr .value .bar {
    height: 6px;
}
.attr-stats .attr .value .bar.baseline {
    background-color: var(--baseline-color);
}
.attr-stats .attr .value .bar.selection {
    background-color: var(--selection-color);
}

.attr-stats .attr-value-details {
    font-size: 12px;
    min-width: 200px;
    max-width: 50%;
}
.attr-stats .attr-value-details .baseline,
.attr-stats .attr-value-details .selection {
    display: flex;
    align-items: center;
    gap: 2px;
}
.attr-stats .attr-value-details .marker {
    height: 12px;
    width: 12px;
}
.attr-stats .attr-value-details .baseline .marker {
    background-color: var(--baseline-color);
}
.attr-stats .attr-value-details .selection .marker {
    background-color: var(--selection-color);
}

.errors:deep(table) {
    table-layout: fixed;
}
.service {
    position: relative;
    padding-left: 8px;
}
.service::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    border-left-width: 4px;
    border-left-style: solid;
    border-left-color: inherit;
}
</style>
