<template>
    <div>
        <v-progress-linear v-if="loading" indeterminate color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <v-alert v-if="rca === 'not implemented'" color="info" outlined text class="mt-5">
            AI-powered Root Cause Analysis is available only in Coroot Enterprise (from $1 per CPU core/month).
            <a href="https://coroot.com/account" target="_blank" class="font-weight-bold">Start</a> your free trial today.
        </v-alert>

        <div v-else-if="rca">
            <template v-if="rca.latency_chart || rca.errors_chart">
                <div class="text-h6">
                    Service Level Indicators of
                    <router-link :to="{ name: 'overview', params: { view: 'applications', id: appId }, query: $utils.contextQuery() }" class="name">
                        {{ $utils.appId(appId).name }}
                    </router-link>
                </div>

                <div class="grey--text mb-2 mt-1">
                    <v-icon size="18" style="vertical-align: baseline">mdi-lightbulb-on-outline</v-icon>
                    Select a chart area to identify the root cause of an anomaly
                </div>

                <v-row>
                    <v-col cols="12" md="6">
                        <Chart
                            v-if="rca.latency_chart"
                            :chart="rca.latency_chart"
                            class="my-5 chart"
                            :loading="loading"
                            @select="explainAnomaly"
                            :selection="selection"
                        />
                    </v-col>
                    <v-col cols="12" md="6">
                        <Chart
                            v-if="rca.errors_chart"
                            :chart="rca.errors_chart"
                            class="my-5 chart"
                            :loading="loading"
                            @select="explainAnomaly"
                            :selection="selection"
                        />
                    </v-col>
                </v-row>
            </template>

            <div v-if="rca.causes && rca.causes.length > 0">
                <div class="mt-5 mb-3 text-h6">Possible causes</div>

                <v-simple-table dense>
                    <thead>
                        <tr>
                            <th>Issue</th>
                            <th>Reasons</th>
                            <th>Applications</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="c in rca.causes">
                            <td class="text-no-wrap">
                                <div>
                                    <v-icon color="error" small class="mr-1">mdi-alert-circle</v-icon>
                                    <span v-html="c.summary" />
                                </div>
                            </td>
                            <td class="text-no-wrap">
                                <ul>
                                    <li v-for="d in c.details">
                                        <span v-html="d" />
                                    </li>
                                </ul>
                            </td>
                            <td>
                                <div class="d-flex flex-wrap">
                                    <template v-for="(s, i) in c.affected_services">
                                        <router-link
                                            :to="{ name: 'overview', params: { view: 'applications', id: s }, query: $utils.contextQuery() }"
                                        >
                                            {{ $utils.appId(s).name }}
                                        </router-link>
                                        <span v-if="i + 1 < c.affected_services.length" class="mr-1">,</span>
                                    </template>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </v-simple-table>
            </div>

            <div v-if="tree.length">
                <div class="mt-5 mb-3 text-h6">Detailed RCA report</div>

                <div>
                    <RcaItem v-for="h in tree" :key="h.id" :hyp="h" :split="70" />
                </div>
            </div>
        </div>
    </div>
</template>

<script>
import RcaItem from '@/components/RcaItem.vue';
import { palette } from '@/utils/colors';
import Chart from '@/components/Chart.vue';

export default {
    computed: {
        tree() {
            if (!this.rca.hypotheses || !this.rca.hypotheses.length) {
                return [];
            }
            const f = (s, parent) => {
                const h = {
                    id: s.id,
                    name: s.name,
                    children: [],
                    level: parent.level + 1,
                    service: s.service,
                    color: palette.hash2(s.service),
                    timeseries: s.timeseries,
                    possible_cause: s.possible_cause,
                    widgets: s.widgets,
                    disable_reason: s.disable_reason,
                    log_pattern: s.log_pattern,
                };
                parent.children.push(h);
                this.rca.hypotheses
                    .filter((s) => s.parent_id === h.id)
                    .forEach((s) => {
                        f(s, h);
                    });
            };
            const root = { level: -1, children: [] };
            f(this.rca.hypotheses[0], root);
            return root.children;
        },
    },
    props: {
        appId: String,
    },

    components: { Chart, RcaItem },

    data() {
        return {
            rca: null,
            loading: false,
            error: '',
            selection: { mode: '', from: this.$route.query.rcaFrom || 0, to: this.$route.query.rcaTo || 0 },
        };
    },

    watch: {
        '$route.query'() {
            this.selection.from = this.$route.query.rcaFrom || 0;
            this.selection.to = this.$route.query.rcaTo || 0;
        },
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    methods: {
        explainAnomaly(s) {
            this.selection.from = s.selection.from;
            this.selection.to = s.selection.to;
            this.$router.push({ query: { ...this.$route.query, rcaFrom: s.selection.from, rcaTo: s.selection.to, ...s.ctx } });
            this.get();
        },
        get() {
            this.loading = true;
            this.$api.getRCA(this.appId, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.rca = data;
            });
        },
    },
};
</script>

<style scoped></style>
