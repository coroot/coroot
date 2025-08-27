<template>
    <Views :loading="loading" :error="error">
        <template v-if="$route.query.incident" #subtitle>{{ $route.query.incident }}</template>

        <CheckForm v-model="editing.active" :appId="editing.appId" :check="editing.check" />

        <div v-if="incident">
            <v-card outlined class="my-6 pa-4 pb-2">
                <div class="text-h6 mb-2">
                    <v-icon :color="incident.severity === 'critical' ? 'error' : 'warning'" style="margin-bottom: 2px">mdi-alert-circle </v-icon>
                    <span>
                        {{ incident.short_description }}
                    </span>
                </div>

                <div class="d-flex flex-wrap" style="gap: 16px; row-gap: 8px">
                    <div>
                        <span class="field-name">Started</span>:
                        <span>
                            {{ $format.date(incident.opened_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}
                        </span>
                        <span> ({{ $format.timeSinceNow(incident.opened_at) }} ago) </span>
                    </div>

                    <div>
                        <span class="field-name">Resolved</span>:
                        <span>
                            <template v-if="incident.resolved_at">
                                {{ $format.date(incident.resolved_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}
                            </template>
                            <span v-else>still open</span>
                        </span>
                    </div>

                    <div>
                        <span class="field-name">Duration</span>:
                        <span>{{ $format.durationPretty(incident.duration) }}</span>
                    </div>

                    <div>
                        <span class="field-name">Application</span>:
                        <router-link
                            :to="{ name: 'overview', params: { view: 'applications', id: incident.application_id }, query: $utils.contextQuery() }"
                            class="name"
                        >
                            {{ $utils.appId(incident.application_id).name }}
                        </router-link>
                    </div>

                    <div>
                        <span class="field-name"> Root Cause Analysis: </span>
                        <template v-if="incident.rca">
                            <span v-if="incident.rca.status === 'OK'" class="green--text">Done</span>
                            <v-tooltip v-else-if="incident.rca.status === 'Failed'" bottom>
                                <template #activator="{ on }">
                                    <span v-on="on" class="red--text">Failed</span>
                                </template>
                                <v-card class="pa-2"> Failed: {{ incident.rca.error }} </v-card>
                            </v-tooltip>
                            <span v-else class="grey--text">{{ incident.rca.status }}</span>
                        </template>
                        <span v-else class="grey--text">&mdash;</span>
                        <v-btn icon small @click="refresh_rca()" :loading="loading"><v-icon small>mdi-refresh</v-icon></v-btn>

                        <a href="https://docs.coroot.com/ai/overview" target="_blank" class="ml-1">
                            <v-icon small>mdi-information-outline</v-icon>
                        </a>
                    </div>
                </div>

                <v-simple-table dense class="mt-5 table">
                    <thead>
                        <tr>
                            <th>Service Level Objective (SLO)</th>
                            <th>Objective</th>
                            <th>Compliance</th>
                            <th>
                                Error budget burn rate
                                <a href="https://docs.coroot.com/alerting/slo-monitoring" target="_blank" class="ml-1"
                                    ><v-icon small>mdi-information-outline</v-icon></a
                                >
                            </th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-if="incident.availability_slo">
                            <td>Availability</td>
                            <td>
                                {{ incident.availability_slo.objective }}
                                <v-btn small icon @click="edit('SLOAvailability', 'Availability')"><v-icon small>mdi-pencil</v-icon></v-btn>
                            </td>
                            <td>
                                <span :class="{ fired: incident.availability_slo.violated }">
                                    {{ incident.availability_slo.compliance }}
                                </span>
                            </td>
                            <td>
                                <template v-if="availabilityBurnRate">
                                    <span class="caption grey&#45;&#45;text">{{ $format.durationPretty(availabilityBurnRate.long_window) }}: </span>
                                    <span :class="{ 'red--text': availabilityBurnRate.long_window_burn_rate > availabilityBurnRate.threshold }">
                                        {{ availabilityBurnRate.long_window_burn_rate.toFixed(0) }}
                                    </span>

                                    <span class="caption grey--text">{{ $format.durationPretty(availabilityBurnRate.short_window) }}: </span>
                                    <span :class="{ 'red--text': availabilityBurnRate.short_window_burn_rate > availabilityBurnRate.threshold }">
                                        {{ availabilityBurnRate.short_window_burn_rate.toFixed(0) }}
                                    </span>

                                    <span class="caption grey--text">threshold: </span>
                                    {{ availabilityBurnRate.threshold.toFixed(0) }}
                                </template>
                            </td>
                        </tr>
                        <tr v-if="incident.latency_slo">
                            <td>Latency</td>
                            <td>
                                {{ incident.latency_slo.objective }}
                                <v-btn small icon @click="edit('SLOLatency', 'Latency')"><v-icon small>mdi-pencil</v-icon></v-btn>
                            </td>
                            <td>
                                <span :class="{ fired: incident.latency_slo.violated }">
                                    {{ incident.latency_slo.compliance }}
                                </span>
                            </td>
                            <td>
                                <template v-if="latencyBurnRate">
                                    <span class="caption grey&#45;&#45;text">{{ $format.durationPretty(latencyBurnRate.long_window) }}: </span>
                                    <span :class="{ 'red--text': latencyBurnRate.long_window_burn_rate > latencyBurnRate.threshold }">
                                        {{ latencyBurnRate.long_window_burn_rate.toFixed(0) }}
                                    </span>

                                    <span class="caption grey--text">{{ $format.durationPretty(latencyBurnRate.short_window) }}: </span>
                                    <span :class="{ 'red--text': latencyBurnRate.short_window_burn_rate > latencyBurnRate.threshold }">
                                        {{ latencyBurnRate.short_window_burn_rate.toFixed(0) }}
                                    </span>

                                    <span class="caption grey--text">threshold: </span>
                                    {{ latencyBurnRate.threshold.toFixed(0) }}
                                </template>
                            </td>
                        </tr>
                    </tbody>
                </v-simple-table>
            </v-card>

            <v-tabs height="32" show-arrows hide-slider>
                <v-tab v-for="v in views" :key="v.name" :to="openView(v.name)" class="view" :class="{ active: view === v.name }">
                    <v-icon small class="mr-1">{{ v.icon }}</v-icon>
                    {{ v.title }}
                </v-tab>
            </v-tabs>

            <template v-if="view === 'overview'">
                <div v-if="incident.rca">
                    <template v-if="incident.rca.root_cause">
                        <div class="mt-5 mb-3 text-h6"><v-icon color="red">mdi-fire</v-icon> Root Cause</div>
                        <Markdown :src="incident.rca.root_cause" :widgets="[]" />

                        <template v-if="incident.rca.detailed_root_cause_analysis">
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
                                <Markdown :src="incident.rca.detailed_root_cause_analysis" :widgets="incident.rca.widgets || []" />
                            </v-card>
                        </template>
                    </template>

                    <template v-if="incident.rca.immediate_fixes">
                        <div class="mt-5 mb-3 text-h6"><v-icon color="red">mdi-fire-extinguisher</v-icon> Immediate Fixes</div>
                        <Markdown :src="incident.rca.immediate_fixes" :widgets="[]" />
                    </template>
                </div>
                <template v-if="incident.widgets">
                    <div class="mt-5 mb-3 text-h6"><v-icon color="red">mdi-chart-bar</v-icon> Service Level Indicators (SLIs)</div>
                    <div class="d-flex flex-wrap mt-5">
                        <Widget
                            v-for="w in incident.widgets"
                            :w="w"
                            class="my-5"
                            :style="{ width: $vuetify.breakpoint.mdAndUp ? w.width || '50%' : '100%' }"
                        />
                    </div>
                </template>
            </template>

            <template v-else-if="view === 'traces'">
                <AppTraces :appId="incident.application_id" compact />
            </template>
        </div>
        <NoData v-else-if="!loading && !error" />
    </Views>
</template>

<script>
import Views from '@/views/Views.vue';
import NoData from '@/components/NoData';
import Widget from '@/components/Widget.vue';
import CheckForm from '@/components/CheckForm.vue';
import AppTraces from '@/views/AppTraces.vue';
import Markdown from '@/components/Markdown.vue';

export default {
    components: { Markdown, Views, AppTraces, CheckForm, Widget, NoData },

    computed: {
        availabilityBurnRate() {
            const rates = this.incident?.details?.availability_burn_rates;
            if (!Array.isArray(rates) || rates.length === 0) {
                return null;
            }
            return rates.find((br) => br.severity !== 'ok') || rates[0];
        },
        latencyBurnRate() {
            const rates = this.incident?.details?.latency_burn_rates;
            if (!Array.isArray(rates) || rates.length === 0) {
                return null;
            }
            return rates.find((br) => br.severity !== 'ok') || rates[0];
        },
        view() {
            return this.$route.query.view || 'overview';
        },
        views() {
            return [
                { name: 'overview', title: 'overview', icon: 'mdi-format-list-checkbox' },
                { name: 'traces', title: 'traces', icon: 'mdi-chart-timeline' },
            ];
        },
    },

    data() {
        return {
            incident: null,
            loading: false,
            error: '',
            editing: {
                active: false,
            },
            show_details: false,
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    methods: {
        get() {
            this.loading = true;
            this.$api.getIncident(this.$route.query.incident, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.incident = data;
            });
        },
        toggle_rca_details() {
            this.show_details = !this.show_details;
        },
        refresh_rca() {
            this.loading = true;
            this.$api.getRCA(this.incident.application_id, true, (data, error) => {
                this.loading = false;
                if (error) {
                    // this.error = error;
                    return;
                }
                this.get();
            });
        },
        edit(check_id, check_title) {
            this.editing = { active: true, appId: this.incident.application_id, check: { id: check_id, title: check_title } };
        },
        openView(v) {
            if (v === 'traces') {
                let durRange = '';
                if (this.incident.latency_slo && this.incident.latency_slo.threshold > 0) {
                    durRange = `${this.incident.latency_slo.threshold}-err`;
                }
                const trace = `::${this.incident.actual_from}-${this.incident.actual_to}:${durRange}:`;
                return { query: { ...this.$route.query, view: v, trace } };
            }
            return { query: { ...this.$route.query, view: v, trace: undefined } };
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

.table:deep(table) {
    min-width: 500px;
}
.table:deep(tr:hover) {
    background-color: unset !important;
}

.fired {
    opacity: 100%;
    border-bottom: 2px solid red !important;
    background-color: unset !important;
}

.field-name {
    font-weight: 700;
    color: var(--text-color-dimmed);
    font-size: 14px;
}
</style>
