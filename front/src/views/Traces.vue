<template>
    <div v-if="view">
        <v-alert v-if="view.message" color="primary" icon="mdi-alert-octagon-outline" outlined text>
            {{ view.message }}
        </v-alert>

        <Heatmap v-if="view.heatmap" :heatmap="view.heatmap" :selection="selection" @select="setSelection" :loading="loading" />

        <v-tabs height="32" hide-slider>
            <v-tab v-for="v in ['traces', 'investigation']" :to="openView(v)" class="view" :class="{ active: query.view === v }">
                {{ v }}
            </v-tab>
        </v-tabs>

        <div v-if="view.heatmap && !query.ts_from" class="grey--text my-3 text-center">
            <v-icon size="20" style="vertical-align: baseline">mdi-lightbulb-on-outline</v-icon>
            <template v-if="query.view === 'investigation'">Select a chart area to explore trace attributes.</template>
            <template v-else>Select a chart area to see traces for a specific time range, duration, or status.</template>
        </div>

        <div v-if="query.trace_id" class="mt-5" style="min-height: 50vh">
            <div class="text-md-h6 mb-3">
                <router-link :to="openView('traces')">
                    <v-icon>mdi-arrow-left</v-icon>
                </router-link>
                Trace {{ query.trace_id }}
            </div>
            <v-progress-linear v-if="loading" indeterminate color="green" height="4" />
            <TracingTrace v-else-if="view.spans" :spans="view.spans" />
        </div>

        <div v-else-if="query.view === 'investigation'" class="mt-5 stats" :style="{ gap: statsStyles.gap }">
            <v-progress-linear v-if="loading" indeterminate color="green" height="4" />
            <div v-for="attr in view.stats" class="attr" :style="{ width: statsStyles.attrWidth }">
                <div class="name">{{ attr.name }}</div>
                <v-tooltip v-for="v in attr.values" bottom transition="none" attach=".stats" content-class="attr-value-details">
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

        <div v-else class="mt-5" style="min-height: 50vh">
            <v-simple-table class="spans">
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
                        <td colspan="6" class="pa-0" style="vertical-align: top">
                            <v-progress-linear v-if="loading" indeterminate color="green" height="4" />
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
                        <td class="text-no-wrap">{{ s.duration.toFixed(2) }}ms</td>
                    </tr>
                </tbody>
            </v-simple-table>
            <div v-if="!loading && (!view.spans || !view.spans.length)" class="pa-3 text-center grey--text">No traces found</div>
            <div v-if="!loading && view.spans && view.spans.length && view.limit" class="text-right caption grey--text">
                The output is capped at {{ view.limit }} traces.
            </div>
        </div>
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
                q.view = 'traces';
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
        setQuery(q, from, to) {
            const hm = this.view.heatmap;
            const rq = this.$route.query;
            const heatmap = hm && rq.from && rq.to && hm.ctx.from === rq.from && hm.ctx.to === rq.to;
            const query = q ? JSON.stringify(q) : undefined;
            return { query: { query, from, to } };
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
            this.$router.push(this.setQuery(q, from, to));
        },
    },
};
</script>

<style scoped>
.view {
    color: var(--text-color-dimmed);
}
.view.active {
    color: var(--text-color);
    border-bottom: 2px solid var(--text-color);
}

.stats {
    --baseline-color: #42a5f5;
    --selection-color: #ffca28;

    display: flex;
    flex-wrap: wrap;
}
.stats .attr {
    border: 1px solid var(--border-color);
    border-radius: 4px;
    font-size: 12px;
    padding: 8px 4px;
}
.stats .attr .name {
    font-weight: 700;
    font-size: 14px;
    color: var(--text-color-dimmed);
    margin-bottom: 8px;
    padding: 0 4px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.stats .attr .value {
    display: flex;
    align-items: center;
    cursor: pointer;
    margin-bottom: 4px;
}
.stats .attr .value:hover {
    background-color: var(--background-color-hi);
}
.stats .attr .value .name {
    font-weight: normal;
    color: var(--text-color);
    margin-bottom: 0;
    width: 60%;
}
.stats .attr .value .bars {
    width: 40%;
}
.stats .attr .value .bar {
    height: 6px;
}
.stats .attr .value .bar.baseline {
    background-color: var(--baseline-color);
}
.stats .attr .value .bar.selection {
    background-color: var(--selection-color);
}

.stats .attr-value-details {
    font-size: 12px;
    min-width: 200px;
    max-width: 50%;
}
.stats .attr-value-details .baseline,
.stats .attr-value-details .selection {
    display: flex;
    align-items: center;
    gap: 2px;
}
.stats .attr-value-details .marker {
    height: 12px;
    width: 12px;
}
.stats .attr-value-details .baseline .marker {
    background-color: var(--baseline-color);
}
.stats .attr-value-details .selection .marker {
    background-color: var(--selection-color);
}
</style>
