<template>
    <div>
        <v-progress-linear indeterminate v-if="loading" color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <ApplicationFilter :applications="applications" @filter="setFilter" class="my-4" />

        <v-data-table
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="50"
            :items="items"
            sort-by="application"
            sort-desc
            must-sort
            no-data-text="No applications found"
            ref="table"
            v-on-resize="calcWidth"
            :headers="[
                { value: 'application', text: 'Application', sortable: false },
                { value: 'requests', text: 'Requests', sortable: false },
                { value: 'latency', text: 'Latency', sortable: false, width: this.sparklineWidthPercent + '%' },
                { value: 'errors', text: 'Errors', sortable: false, width: this.sparklineWidthPercent + '%' },
                { value: 'incident', text: 'Incident', sortable: false, width: '14ch' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
        >
            <template #item.application="{ item }">
                <div class="application">
                    <div class="name">
                        <router-link :to="{ name: 'overview', params: { view: 'anomalies', id: item.id }, query: $utils.contextQuery() }">
                            {{ $utils.appId(item.id).name }}
                        </router-link>
                    </div>
                </div>
            </template>

            <template #item.incident="{ item }">
                <div v-if="item.incident" class="incident">
                    <div class="status" :class="statuses[item.incident.resolved_at ? 'ok' : item.incident.severity].color" />
                    <router-link :to="{ name: 'overview', params: { view: 'incidents' }, query: { incident: item.incident.key } }">
                        <span class="key" style="font-family: monospace">i-{{ item.incident.key }}</span>
                    </router-link>
                </div>
                <span v-else class="grey--text">-</span>
            </template>

            <template #item.requests="{ item }">
                <div v-if="item.rps">
                    {{ $format.float(item.rps) }}
                    <span class="grey--text caption">/s</span>
                </div>
                <span v-else class="grey--text">-</span>
            </template>

            <template #item.errors="{ item }">
                <div v-if="item.errors">
                    <router-link :to="{ name: 'overview', params: { view: 'anomalies', id: item.id }, query: $utils.contextQuery() }" class="chart">
                        <div v-if="item.errors.msg" class="value">{{ item.errors.msg }}</div>
                        <v-sparkline
                            v-if="item.errors.chart"
                            :value="item.errors.chart.map((v) => (v === null ? 0 : v))"
                            fill
                            smooth
                            padding="4"
                            :color="`${item.errors.msg ? 'red' : 'grey'} ${$vuetify.theme.dark ? '' : 'lighten-2'}`"
                            height="30"
                            :width="sparklineWidth"
                        />
                    </router-link>
                </div>
            </template>

            <template #item.latency="{ item }">
                <div v-if="item.latency">
                    <router-link :to="{ name: 'overview', params: { view: 'anomalies', id: item.id }, query: $utils.contextQuery() }" class="chart">
                        <div v-if="item.latency.msg" class="value">{{ item.latency.msg }}</div>
                        <v-sparkline
                            v-if="item.latency.chart"
                            :value="item.latency.chart.map((v) => (v === null ? 0 : v))"
                            fill
                            smooth
                            padding="4"
                            :color="`${item.latency.msg ? 'red' : 'grey'} ${$vuetify.theme.dark ? '' : 'lighten-2'}`"
                            height="30"
                            :width="sparklineWidth"
                        />
                    </router-link>
                </div>
            </template>
        </v-data-table>
    </div>
</template>

<script>
import ApplicationFilter from '../components/ApplicationFilter.vue';

export default {
    components: { ApplicationFilter },

    data() {
        return {
            loading: false,
            error: '',
            statuses: {
                critical: { name: 'Critical', color: 'red lighten-1' },
                warning: { name: 'Warning', color: 'orange lighten-1' },
                ok: { name: 'Resolved', color: 'grey lighten-1' },
            },
            anomalyApplications: [],
            filter: new Set(),
            sparklineWidth: 120,
            sparklineWidthPercent: 30,
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    computed: {
        applications() {
            if (!this.anomalyApplications) {
                return [];
            }
            const applications = {};
            this.anomalyApplications.forEach((i) => {
                applications[i.id] = i.category;
            });
            return Object.keys(applications).map((id) => ({ id, category: applications[id] }));
        },
        items() {
            if (!this.anomalyApplications) {
                return [];
            }
            return this.anomalyApplications.filter((i) => this.filter.has(i.id));
        },
    },

    methods: {
        calcWidth() {
            this.sparklineWidth = (this.$refs.table.$el.clientWidth * this.sparklineWidthPercent) / 100;
        },
        get() {
            this.loading = true;
            const query = this.$route.query.query || '';
            this.$api.getOverview('anomalies', query, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.anomalyApplications = data || [];
            });
        },
        setFilter(filter) {
            this.filter = filter;
        },
    },
};
</script>

<style scoped>
.table:deep(table) {
    min-width: 500px;
}
.table:deep(tr:hover) {
    background-color: unset !important;
}
.table:deep(th),
.table:deep(td) {
    padding: 4px 8px !important;
}
.table:deep(th) {
    white-space: nowrap;
}
.table .application {
    display: flex;
    gap: 4px;
}
.table .application .name {
    max-width: 30ch;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.table .chart {
    display: block;
    position: relative;
    height: 100%;
    width: 100%;
    color: inherit;
}
.table .chart .value {
    opacity: 60%;
    position: absolute;
    top: 0;
    width: 100%;
    height: 100%;
    white-space: nowrap;
    display: flex;
    align-items: center;
    justify-content: center;
}

.table:deep(td:has(.chart)) {
    padding: 0 !important;
}
.incident {
    gap: 4px;
    display: flex;
}
.incident .status {
    height: 20px;
    width: 4px;
}

.incident .key {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>
