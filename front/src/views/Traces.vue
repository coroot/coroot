<template>
    <div v-if="view" class="traces">
        <v-alert v-if="view.message" color="info" outlined text class="message">
            <template v-if="view.message === 'not_found'">
                This page only shows traces from OpenTelemetry integrations, not from eBPF. To add OpenTelemetry SDKs to your apps, check out the docs
                for
                <a href="https://coroot.com/docs/coroot-community-edition/tracing/opentelemetry-go" target="_blank">Go</a>,
                <a href="https://coroot.com/docs/coroot-community-edition/tracing/opentelemetry-java" target="_blank">Java</a>,
                <a href="https://coroot.com/docs/coroot-community-edition/tracing/opentelemetry-python" target="_blank">Python</a>.
            </template>
            <template v-if="view.message === 'no_clickhouse'"> Clickhouse integration is not configured. </template>
        </v-alert>

        <template v-else>
            <v-alert v-if="view.error" color="error" icon="mdi-alert-octagon-outline" outlined text>
                {{ view.error }}
            </v-alert>

            <Heatmap v-if="view.heatmap" :heatmap="view.heatmap" :selection="selection" @select="setSelection" :loading="loading" />

            <v-card outlined class="query px-4 py-2 mb-4">
                <div v-if="query.service_name || query.span_name" class="mt-2 d-flex align-center flex-wrap" style="gap: 8px">
                    <div>Where:</div>
                    <v-chip v-if="query.service_name" @click:close="push(filterTraces(undefined, query.span_name))" label close color="primary">
                        Root Service Name = {{ query.service_name }}
                    </v-chip>
                    <v-chip v-if="query.span_name" @click:close="push(filterTraces(query.service_name, undefined))" label close color="primary">
                        Root Span Name = {{ query.span_name }}
                    </v-chip>
                </div>
                <div class="d-flex align-center">
                    <div><div class="marker selection"></div></div>
                    <div>
                        Selection:
                        <template v-if="query.dur_to">
                            time <var> {{ format(query.ts_from, 'ts') }}</var> — <var> {{ format(query.ts_to, 'ts') }}</var>
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
                        </template>
                        <span v-else class="grey--text">
                            <template v-if="query.view === 'investigation'">select a chart area to explore trace attributes</template>
                            <template v-else>select a chart area to see traces for a specific time range, duration, or status</template>
                        </span>
                    </div>
                </div>
                <div v-if="query.view === 'investigation'" class="d-flex align-center">
                    <div><div class="marker baseline"></div></div>
                    Baseline: all other events within the time window
                </div>
                <v-form :disabled="loading">
                    <v-checkbox
                        v-model="form.excludeAux"
                        label="Exclude auxiliary requests (from monitoring, control plane, etc)"
                        dense
                        hide-details
                    />
                </v-form>
            </v-card>

            <v-tabs height="32" hide-slider>
                <v-tab v-for="v in ['overview', 'traces', 'investigation']" :to="openView(v)" class="view" :class="{ active: query.view === v }">
                    {{ v }}
                </v-tab>
            </v-tabs>

            <div v-if="query.trace_id" class="mt-5" style="min-height: 50vh">
                <div class="text-md-h6 mb-3">
                    <router-link :to="openView('traces')">
                        <v-icon>mdi-arrow-left</v-icon>
                    </router-link>
                    Trace {{ query.trace_id }}
                </div>
                <v-progress-linear v-if="loading" indeterminate height="4" />
                <TracingTrace v-else-if="view.spans" :spans="view.spans" />
            </div>

            <div v-else-if="query.view === 'overview'" class="mt-5" style="min-height: 50vh">
                <v-data-table
                    :items="view.summary ? view.summary.stats : []"
                    :loading="loading"
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
                                    <span>{{ format(item.failed, '%') }}</span>
                                    <span class="caption grey--text">%</span>
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
                <v-simple-table dense class="spans">
                    <thead>
                        <tr>
                            <th></th>
                            <th>Root Service</th>
                            <th>Name</th>
                            <th>Status</th>
                            <th>Duration</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-if="loading">
                            <td colspan="5" class="pa-0" style="vertical-align: top">
                                <v-progress-linear v-if="loading" indeterminate height="4" />
                            </td>
                        </tr>
                        <tr v-else v-for="s in view.spans">
                            <td>
                                <v-btn small icon :to="openTrace(s.trace_id)" exact>
                                    <v-icon small>mdi-chart-timeline</v-icon>
                                </v-btn>
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
                <div v-if="!loading && (!view.spans || !view.spans.length)" class="pa-3 text-center grey--text">No traces found</div>
                <div v-if="!loading && view.spans && view.spans.length && view.limit" class="text-right caption grey--text">
                    The output is capped at {{ view.limit }} traces.
                </div>
            </div>

            <div v-else-if="query.view === 'investigation'">
                <div class="grey--text mt-2 mb-3">
                    <v-icon small class="mb-1">mdi-information-outline</v-icon>
                    This section shows how the attributes of traces in the selected area differ from those of other traces.
                </div>
                <v-progress-linear v-if="loading" indeterminate height="4" class="mb-1" />
                <div class="attr-stats" :style="{ gap: statsStyles.gap }">
                    <div v-for="attr in view.attr_stats" class="attr" :style="{ width: statsStyles.attrWidth }">
                        <div class="name">{{ attr.name }}</div>
                        <v-tooltip v-for="v in attr.values" bottom transition="none" attach=".attr-stats" content-class="attr-value-details">
                            <template #activator="{ on }">
                                <div class="value" v-on="on">
                                    <div class="name">
                                        {{ v.name }}
                                    </div>
                                    <div class="bars">
                                        <div class="bar baseline" :style="{ width: v.baseline * 100 + '%' }"></div>
                                        <div class="bar selection" :style="{ width: v.selection * 100 + '%' }"></div>
                                    </div>
                                </div>
                            </template>
                            <v-card class="pa-2">
                                <div>Value:</div>
                                <div class="font-weight-medium mb-1">{{ v.name }}</div>
                                <div class="baseline"><span class="marker" />Baseline: {{ (v.baseline * 100).toFixed(1) }}%</div>
                                <div class="selection"><span class="marker" />Selection: {{ (v.selection * 100).toFixed(1) }}%</div>
                            </v-card>
                        </v-tooltip>
                    </div>
                </div>
            </div>
        </template>
    </div>
</template>

<script>
import Heatmap from '@/components/Heatmap.vue';
import TracingTrace from '@/components/TracingTrace.vue';

export default {
    props: {
        view: Object,
        loading: Boolean,
    },

    components: { TracingTrace, Heatmap },

    data() {
        return {
            form: {
                excludeAux: true,
            },
        };
    },

    mounted() {
        this.form.excludeAux = !this.query.include_aux;
    },

    watch: {
        form: {
            handler(v) {
                this.setForm(v);
            },
            deep: true,
        },
    },

    computed: {
        views() {
            return {
                traces: true,
                investigation: this.query.ts_from && this.query.ts_to,
            };
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
        selection() {
            const q = this.query;
            return { x1: q.ts_from, x2: q.ts_to, y1: q.dur_from, y2: q.dur_to };
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
            q.service_name = serviceName;
            q.span_name = spanName;
            if (spanName) {
                q.view = 'traces';
            }
            if (errors) {
                const { from, to } = this.view.heatmap.ctx;
                q.ts_from = from;
                q.ts_to = to;
                q.dur_from = 'inf';
                q.dur_to = 'err';
            }
            return this.setQuery(q, from, to);
        },
        openTrace(id) {
            const { from, to } = this.$route.query;
            const q = { ...this.query };
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
        setForm(f) {
            const { from, to } = this.$route.query;
            const q = { ...this.query };
            q.include_aux = !f.excludeAux;
            this.push(this.setQuery(q, from, to));
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

.table:deep(table) {
    min-width: 500px;
}
.table:deep(tr:hover) {
    background-color: unset !important;
}
.table:deep(th),
.table:deep(td) {
    padding: 0 8px !important;
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
</style>
