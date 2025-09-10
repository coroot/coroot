<template>
    <Views :loading="loading" :error="error" :noTitle="noTitle">
        <template #subtitle>{{ $utils.appId(appId).name }}</template>

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

            <div v-if="rca.summary">
                <template v-if="rca.summary.root_cause">
                    <div class="mt-5 mb-3 text-h6"><v-icon color="red">mdi-fire</v-icon> Root Cause</div>
                    <Markdown :src="rca.summary.root_cause" :widgets="[]" />

                    <template v-if="rca.summary.detailed_root_cause_analysis">
                        <div>
                            <a @click="toggle_rca_details"
                                >Show
                                <template v-if="!show_details">more</template>
                                <template v-else>less</template>
                                details

                                <v-icon v-if="!show_details">mdi-chevron-down</v-icon>
                                <v-icon v-else>mdi-chevron-up</v-icon>
                            </a>
                        </div>

                        <v-card outlined v-if="show_details" class="pa-5 mt-5">
                            <Markdown :src="rca.summary.detailed_root_cause_analysis" :widgets="rca.summary.widgets || []" />
                        </v-card>
                    </template>
                </template>

                <template v-if="rca.summary.immediate_fixes">
                    <div class="mt-5 mb-3 text-h6"><v-icon color="red">mdi-fire-extinguisher</v-icon> Immediate Fixes</div>
                    <Markdown :src="rca.summary.immediate_fixes" :widgets="[]" />
                </template>
            </div>

            <div v-else-if="rca.ai_integration_enabled">
                <div class="pa-5" style="position: relative; border-radius: 4px">
                    <div style="filter: blur(5px)">
                        <v-skeleton-loader boilerplate type="article, text"></v-skeleton-loader>
                    </div>

                    <v-overlay absolute opacity="0.1" z-index="1">
                        <v-btn color="primary" @click="get('true')" class="mx-auto" :loading="loading">
                            <v-icon small left>mdi-creation</v-icon>
                            Investigate with AI
                        </v-btn>
                    </v-overlay>
                </div>
            </div>
            <div v-else>
                <div class="pa-5" style="position: relative; border-radius: 4px">
                    <div style="filter: blur(7px)">
                        <v-skeleton-loader boilerplate type="article, text"></v-skeleton-loader>
                    </div>

                    <v-overlay absolute opacity="0.1">
                        <v-btn color="primary" :to="{ name: 'project_settings', params: { tab: 'ai' } }" class="mx-auto">
                            <v-icon small left>mdi-creation</v-icon>
                            Enable an AI integration
                        </v-btn>
                    </v-overlay>
                </div>
            </div>
        </div>
    </Views>
</template>

<script>
import { palette } from '@/utils/colors';
import Chart from '@/components/Chart.vue';
import Views from '@/views/Views.vue';
import Markdown from '@/components/Markdown.vue';

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
        noTitle: Boolean,
    },

    components: { Markdown, Views, Chart },

    data() {
        return {
            rca: null,
            loading: false,
            show_details: false,
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
        toggle_rca_details() {
            this.show_details = !this.show_details;
        },
        explainAnomaly(s) {
            this.selection.from = s.selection.from;
            this.selection.to = s.selection.to;
            this.$router.push({ query: { ...this.$route.query, rcaFrom: s.selection.from, rcaTo: s.selection.to, ...s.ctx } });
            this.get();
        },
        get(withSummary) {
            this.loading = true;
            this.$api.getRCA(this.appId, withSummary, (data, error) => {
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

<style scoped>
.summary >>> h3 {
    margin: 16px 0 16px 0;
}
</style>
