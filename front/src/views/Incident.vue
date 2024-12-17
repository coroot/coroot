<template>
    <div>
        <v-progress-linear v-if="loading" indeterminate color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <CheckForm v-model="editing.active" :appId="editing.appId" :check="editing.check" />

        <div v-if="incident">
            <v-card outlined class="my-6 pa-4 pb-2">
                <div class="d-flex flex-wrap" style="gap: 16px; row-gap: 8px">
                    <div>
                        <span class="field-name">Incident</span>:
                        <span>i-{{ $route.query.incident }}</span>
                    </div>

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
                        <span class="field-name">Severity</span>:
                        <span>
                            <v-icon :color="incident.severity === 'critical' ? 'error' : 'warning'" small style="margin-bottom: 2px">
                                mdi-alert-circle
                            </v-icon>
                            <span class="text-uppercase">{{ incident.severity }}</span>
                        </span>
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
                </div>

                <v-simple-table dense class="mt-5 table">
                    <thead>
                        <tr>
                            <th>Service Level Objective (SLO)</th>
                            <th>Objective</th>
                            <th>Compliance</th>
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
                <template v-if="incident.heatmap">
                    <div class="d-flex flex-wrap mt-5">
                        <Widget
                            :w="incident.heatmap"
                            class="my-5"
                            :style="{ width: $vuetify.breakpoint.mdAndUp ? incident.heatmap.width || '50%' : '100%' }"
                        />
                    </div>
                </template>
            </template>

            <template v-else-if="view === 'traces'">
                <Tracing :appId="incident.application_id" compact />
            </template>

            <template v-else-if="view === 'rca'">
                <RCA :appId="incident.application_id" />
            </template>
        </div>
        <NoData v-else-if="!loading && !error" />
    </div>
</template>

<script>
import NoData from '@/components/NoData';
import Widget from '@/components/Widget.vue';
import CheckForm from '@/components/CheckForm.vue';
import Tracing from '@/views/Tracing.vue';
import RCA from '@/views/RCA.vue';

export default {
    components: { Tracing, CheckForm, Widget, RCA, NoData },

    computed: {
        view() {
            return this.$route.query.view || 'overview';
        },
        views() {
            return [
                { name: 'overview', title: 'overview', icon: 'mdi-format-list-checkbox' },
                { name: 'traces', title: 'traces', icon: 'mdi-chart-timeline' },
                { name: 'rca', title: 'root cause analysis', icon: 'mdi-creation' },
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
