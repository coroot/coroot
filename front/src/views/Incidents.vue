<template>
    <Views :loading="loading" :error="error">
        <ApplicationFilter :applications="applications" @filter="setFilter" @search="setSearch" class="mb-4" />

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
            :items-per-page="limit"
            :items="items"
            sort-by="opened_at"
            sort-desc
            must-sort
            no-data-text="No incidents found"
            :headers="[
                { value: 'incident', text: 'Incident', sortable: false },
                { value: 'application', text: 'Application', sortable: false },
                { value: 'description', text: 'Description', sortable: false },
                { value: 'rca', text: 'Root Cause', sortable: false },
                { value: 'impact', text: 'Impacted requests', sortable: true },
                { value: 'opened_at', text: 'Opened at', sortable: true },
                { value: 'duration', text: 'Duration', sortable: true },
                { value: 'actions', text: '', sortable: false, align: 'end', width: '20px' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100] }"
            @update:items-per-page="changeLimit"
        >
            <template #header.rca>
                <div class="d-flex align-center gap-1">
                    Root Cause
                    <a href="https://docs.coroot.com/ai/overview" target="_blank">
                        <v-icon small>mdi-information-outline</v-icon>
                    </a>
                </div>
            </template>
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

            <template #item.description="{ item }">
                <div class="d-flex">
                    <span :class="{ 'grey--text': item.resolved_at }">
                        {{ item.short_description }}
                    </span>
                </div>
            </template>

            <template #item.rca="{ item }">
                <div>
                    <template v-if="item.rca">
                        <v-icon v-if="item.rca.status === 'OK'" small color="success">mdi-check-circle</v-icon>
                        <v-icon v-else-if="item.rca.status === 'Failed'" small color="error">mdi-alert-circle</v-icon>
                        <span v-else class="grey--text">{{ item.rca.status }}</span>
                    </template>
                    <span v-else class="grey--text">&mdash;</span>
                </div>
            </template>

            <template #item.impact="{ item }">
                <v-progress-linear :value="item.impact" color="red lighten-1" height="16px"> {{ $format.percent(item.impact) }} % </v-progress-linear>
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
                    </v-list>
                </v-menu>
            </template>
        </v-data-table>
    </Views>
</template>

<script>
import Views from '@/views/Views.vue';
import ApplicationFilter from '../components/ApplicationFilter.vue';
import CheckForm from '@/components/CheckForm.vue';

const statuses = {
    critical: { name: 'Critical', color: 'red lighten-1' },
    warning: { name: 'Warning', color: 'orange lighten-1' },
    ok: { name: 'Resolved', color: 'grey lighten-1' },
};

export default {
    components: { Views, CheckForm, ApplicationFilter },

    data() {
        return {
            limit: Number(this.$route.query.limit) || 50,
            incidents: [],
            filter: new Set(),
            search: '',
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
            const search = (this.search || '').trim().toLowerCase();
            let filtered;
            if (search) {
                filtered = this.incidents.filter(
                    (i) => i.short_description.toLowerCase().includes(search) || i.application_id.toLowerCase().includes(search),
                );
            } else {
                filtered = this.incidents.filter((i) => this.filter.has(i.application_id));
            }
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
            this.$api.getIncidents(this.limit, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.incidents = data || [];
            });
        },
        changeLimit(limit) {
            this.limit = limit;
            this.get();
            this.$router.push({ query: { ...this.$route.query, limit } }).catch((err) => err);
        },
        changeShowResolved() {
            this.showResolved = !this.showResolved;
            this.$router.push({ query: { ...this.$route.query, show_resolved: this.showResolved ? '1' : '0' } }).catch((err) => err);
        },
        setFilter(filter) {
            this.filter = filter;
        },
        setSearch(search) {
            this.search = search;
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
