<template>
    <div>
        <v-progress-linear indeterminate v-if="loading" color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <ApplicationFilter :applications="applications" @filter="setFilter" class="my-4" />

        <div class="legend mb-3">
            <div v-for="s in statuses" class="item">
                <div class="count" :class="s.color">{{ s.count }}</div>
                <div class="label">{{ s.name }}</div>
            </div>
            <v-checkbox
                label="Show resolved"
                :value="showResolved"
                @change="changeShowResolved"
                class="font-weight-regular mt-0 pt-0 ml-2"
                style="margin-left: -4px"
                color="primary"
                hide-details
            />
        </div>

        <CheckForm v-model="editing.active" :appId="editing.appId" :check="editing.check" />

        <v-data-table
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="50"
            :items="items"
            sort-by="opened_at"
            sort-desc
            must-sort
            no-data-text="No incidents found"
            :headers="[
                { value: 'incident', text: 'Incident', sortable: false },
                { value: 'application', text: 'Application', sortable: false },
                { value: 'opened_at', text: 'Opened at', sortable: true },
                { value: 'duration', text: 'Duration', sortable: true },
                { value: 'availability', text: 'Availability', sortable: false },
                { value: 'latency', text: 'Latency', sortable: false },
                { value: 'affected_request_percent', text: 'Affected requests', sortable: true },
                { value: 'error_budget_consumed_percent', text: 'Consumed error budged', sortable: true },
                { value: 'actions', text: '', sortable: false, align: 'end', width: '20px' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
        >
            <template #item.incident="{ item }">
                <div class="incident" :class="{ 'grey--text': item.resolved_at }">
                    <div class="status" :class="item.color" />
                    <router-link :to="{ name: 'overview', params: { view: 'incidents' }, query: { ...$utils.contextQuery(), incident: item.key } }">
                        <span class="key" style="font-family: monospace">i-{{ item.key }}</span>
                    </router-link>
                </div>
            </template>

            <template #item.opened_at="{ item }">
                <div class="d-flex text-no-wrap" :class="{ 'grey--text': item.resolved_at }">
                    {{ $format.date(item.opened_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}
                    ({{ $format.timeSinceNow(item.opened_at) }} ago)
                </div>
            </template>

            <template #item.duration="{ item }">
                <div class="d-flex text-no-wrap" :class="{ 'grey--text': item.resolved_at }">
                    {{ $format.durationPretty(item.duration) }}
                </div>
            </template>

            <template #item.application="{ item }">
                <div class="d-flex">
                    <span :class="{ 'grey--text': item.resolved_at }">
                        {{ $utils.appId(item.application_id).name }}
                    </span>
                </div>
            </template>

            <template #item.latency="{ item }">
                <span v-if="item.latency_slo" :class="item.latency_slo.violated ? 'fired' : undefined">
                    {{ item.latency_slo.compliance }}
                </span>
            </template>

            <template #item.availability="{ item }">
                <span v-if="item.availability_slo" :class="item.availability_slo.violated ? 'fired' : undefined">
                    {{ item.availability_slo.compliance }}
                </span>
            </template>

            <template #item.affected_request_percent="{ item }">
                <v-progress-linear :value="item.affected_request_percent" color="blue lighten-1" height="16px">
                    {{ $format.percent(item.affected_request_percent) }} %
                </v-progress-linear>
            </template>

            <template #item.error_budget_consumed_percent="{ item }">
                <v-progress-linear :value="item.error_budget_consumed_percent" color="purple lighten-1" height="16px">
                    {{ $format.percent(item.error_budget_consumed_percent) }} %
                </v-progress-linear>
            </template>
            <template #item.actions="{ item }">
                <v-menu offset-y>
                    <template v-slot:activator="{ attrs, on }">
                        <v-btn icon x-small class="ml-1" v-bind="attrs" v-on="on">
                            <v-icon small>mdi-dots-vertical</v-icon>
                        </v-btn>
                    </template>

                    <v-list dense>
                        <v-list-item @click="edit(item.application_id, 'SLOAvailability', 'Availability')">
                            <v-icon small class="mr-1">mdi-check-circle-outline</v-icon> Adjust Availability SLO
                        </v-list-item>
                        <v-list-item @click="edit(item.application_id, 'SLOLatency', 'Latency')">
                            <v-icon small class="mr-1">mdi-timer-outline</v-icon> Adjust Latency SLO
                        </v-list-item>
                        <v-list-item
                            :to="{
                                name: 'overview',
                                params: { view: 'incidents' },
                                query: { incident: item.key, view: 'rca' },
                            }"
                        >
                            <v-icon small class="mr-1">mdi-creation</v-icon> Investigate with AI
                        </v-list-item>
                    </v-list>
                </v-menu>
            </template>
        </v-data-table>
    </div>
</template>

<script>
import ApplicationFilter from '../components/ApplicationFilter.vue';
import CheckForm from '@/components/CheckForm.vue';

const statuses = {
    critical: { name: 'Critical', color: 'red lighten-1' },
    warning: { name: 'Warning', color: 'orange lighten-1' },
    ok: { name: 'Resolved', color: 'grey lighten-1' },
};

export default {
    components: { CheckForm, ApplicationFilter },

    data() {
        return {
            incidents: [],
            filter: new Set(),
            showResolved: false,
            editing: {
                active: false,
            },
            loading: false,
            error: '',
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
        this.showResolved = this.$route.query.show_resolved === '1';
    },

    watch: {
        items() {
            if (this.items.some((i) => i.resolved_at) && !this.showResolved) {
                this.showResolved = true;
            }
        },
    },

    computed: {
        applications() {
            if (!this.incidents) {
                return [];
            }
            const applications = {};
            this.incidents.forEach((i) => {
                applications[i.application_id] = i.application_category;
            });
            return Object.keys(applications).map((id) => ({ id, category: applications[id] }));
        },
        items() {
            if (!this.incidents) {
                return [];
            }
            let filtered = this.incidents.filter((i) => this.filter.has(i.application_id));
            const shr = this.$route.query.show_resolved;
            if (shr === '0') {
                filtered = filtered.filter((i) => !i.resolved_at);
            }
            if (shr === undefined) {
                const unresolved = filtered.filter((i) => !i.resolved_at);
                if (unresolved.length) {
                    filtered = unresolved;
                }
            }
            return filtered.map((i) => {
                return {
                    ...i,
                    color: statuses[i.resolved_at ? 'ok' : i.severity].color,
                };
            });
        },
        statuses() {
            return Object.keys(statuses).map((s) => {
                return {
                    ...statuses[s],
                    count: this.incidents.filter((i) => (i.resolved_at ? 'ok' : i.severity) === s).length,
                };
            });
        },
    },
    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getOverview('incidents', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.incidents = data.incidents || [];
            });
        },
        changeShowResolved() {
            this.showResolved = !this.showResolved;
            this.$router.push({ query: { ...this.$route.query, show_resolved: this.showResolved ? '1' : '0' } }).catch((err) => err);
        },
        setFilter(filter) {
            this.filter = filter;
        },
        edit(app_id, check_id, check_title) {
            this.editing = { active: true, appId: app_id, check: { id: check_id, title: check_title } };
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

.table:deep(td:has(.incident)) {
    padding-left: 0 !important;
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

.legend {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    align-items: center;
    font-weight: 500;
    font-size: 14px;
}
.legend .item {
    display: flex;
    gap: 4px;
}
.legend .count {
    padding: 0 4px;
    border-radius: 2px;
    height: 18px;
    line-height: 18px;
    color: rgba(255, 255, 255, 0.8);
}
.legend .label {
    opacity: 60%;
}
.fired {
    opacity: 100%;
    border-bottom: 2px solid red !important;
    background-color: unset !important;
}
</style>
