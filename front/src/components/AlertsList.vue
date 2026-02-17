<template>
    <div>
        <div class="d-flex align-center mb-4">
            <v-text-field
                v-model="searchInput"
                label="search"
                clearable
                hide-details
                dense
                outlined
                prepend-inner-icon="mdi-magnify"
                style="max-width: 300px"
                @input="debouncedSearch"
                @click:clear="clearSearch"
            />
        </div>

        <div class="legend mb-3">
            <div v-for="s in statuses" :key="s.name" class="item">
                <div class="count" :class="s.color">{{ s.count }}</div>
                <div class="label">{{ s.name }}</div>
            </div>
            <v-checkbox
                label="Show resolved"
                :input-value="showResolved"
                @click="changeShowResolved"
                class="font-weight-regular mt-0 pt-0 ml-2"
                style="margin-left: -4px"
                color="primary"
                hide-details
            />
        </div>

        <div v-if="selectedFiring.length || selectedSuppressible.length || selectedReopenable.length" class="d-flex align-center justify-end mb-3" style="gap: 8px">
            <v-btn v-if="selectedFiring.length" small outlined @click="resolveSelected" title="Acknowledge the alert; it will reopen if the condition recurs">
                <v-icon small class="mr-1">mdi-check-circle-outline</v-icon>
                Resolve ({{ selectedFiring.length }})
            </v-btn>
            <v-btn v-if="selectedSuppressible.length" small outlined @click="suppressSelected" title="Permanently silence the alert; it will not reopen until manually reopened">
                <v-icon small class="mr-1">mdi-bell-off-outline</v-icon>
                Suppress ({{ selectedSuppressible.length }})
            </v-btn>
            <v-btn v-if="selectedReopenable.length" small outlined @click="reopenSelected" title="Reopen the alert so it can fire again">
                <v-icon small class="mr-1">mdi-restore</v-icon>
                Reopen ({{ selectedReopenable.length }})
            </v-btn>
        </div>

        <v-data-table
            dense
            class="table"
            mobile-breakpoint="0"
            :server-items-length="serverTotal"
            :items-per-page.sync="limit"
            :page.sync="page"
            :items="items"
            :sort-by.sync="sortBy"
            :sort-desc.sync="sortDesc"
            must-sort
            no-data-text="No alerts found"
            :headers="[
                { value: 'select', text: '', sortable: false, width: '40px' },
                { value: 'application', text: 'Application', sortable: true, width: '120px' },
                { value: 'summary', text: 'Summary', sortable: true },
                { value: 'notifications', text: 'Notifications', sortable: false },
                { value: 'opened_at', text: 'Opened at', sortable: true },
                { value: 'resolved_at', text: 'Resolved at', sortable: true },
                { value: 'duration', text: 'Duration', sortable: true },
                { value: 'rule', text: 'Rule', sortable: true },
                { value: 'actions', text: '', sortable: false, align: 'end', width: '20px' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100] }"
            @update:options="onOptionsChange"
        >
            <template #header.select>
                <v-checkbox
                    :value="allSelected"
                    :indeterminate="someSelected && !allSelected"
                    @change="toggleSelectAll"
                    hide-details
                    class="mt-0 pt-0"
                    color="primary"
                />
            </template>

            <template #item.select="{ item }">
                <v-checkbox :value="selected.includes(item.id)" @change="toggleSelect(item.id)" hide-details class="mt-0 pt-0" color="primary" />
            </template>

            <template #item.rule="{ item }">
                <div class="d-flex" :class="{ 'grey--text': item.resolved_at || item.manually_resolved_at || item.suppressed }">
                    <router-link
                        v-if="item.rule_id"
                        :to="{ name: 'overview', params: { view: 'alerts', id: 'rules' }, query: { ...$utils.contextQuery(), rule: item.rule_id } }"
                    >
                        {{ item.rule_name || item.rule_id }}
                    </router-link>
                    <span v-else>-</span>
                </div>
            </template>

            <template #item.opened_at="{ item }">
                <div class="d-flex text-no-wrap" :class="{ 'grey--text': item.resolved_at || item.manually_resolved_at || item.suppressed }">
                    {{ $format.date(item.opened_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}
                    ({{ $format.timeSinceNow(item.opened_at) }} ago)
                </div>
            </template>

            <template #item.resolved_at="{ item }">
                <div v-if="item.suppressed" class="text-no-wrap grey--text">
                    <div>suppressed</div>
                    <div v-if="item.resolved_by" class="caption">by {{ item.resolved_by }}</div>
                </div>
                <div v-else-if="item.manually_resolved_at" class="text-no-wrap grey--text">
                    <div>{{ $format.date(item.manually_resolved_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}</div>
                    <div v-if="item.resolved_by" class="caption">by {{ item.resolved_by }}</div>
                </div>
                <div v-else-if="item.resolved_at" class="text-no-wrap grey--text">
                    <div>{{ $format.date(item.resolved_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}</div>
                </div>
                <span v-else class="grey--text">-</span>
            </template>

            <template #item.duration="{ item }">
                <div class="d-flex text-no-wrap" :class="{ 'grey--text': item.resolved_at || item.manually_resolved_at || item.suppressed }">
                    {{ $format.durationPretty(item.duration) }}
                </div>
            </template>

            <template #item.application="{ item }">
                <div class="app-name" :title="$utils.appId(item.application_id).name">
                    <router-link
                        :to="{
                            name: 'overview',
                            params: { view: 'applications', id: item.application_id, report: item.report || undefined },
                            query: { ...alertContextQuery(item), ...(item.report === 'Logs' ? { query: JSON.stringify({ view: 'patterns', source: 'agent' }) } : {}) },
                        }"
                        :class="{ 'grey--text': item.resolved_at || item.manually_resolved_at || item.suppressed }"
                    >
                        {{ $utils.appId(item.application_id).name }}
                    </router-link>
                </div>
            </template>

            <template #item.summary="{ item }">
                <div class="summary-cell" :class="{ 'grey--text': item.resolved_at || item.manually_resolved_at || item.suppressed }">
                    <div class="severity-bar" :class="item.color" />
                    <div class="summary-content" @click="openAlert(item.id)">
                        <div class="summary-link">{{ item.summary }}</div>
                        <div v-if="codeSample(item)" class="log-sample">{{ codeSample(item) }}</div>
                    </div>
                </div>
            </template>

            <template #item.notifications="{ item }">
                <div class="d-flex flex-wrap" style="gap: 4px" :class="{ 'grey--text': item.resolved_at || item.manually_resolved_at || item.suppressed }">
                    <span
                        v-for="(n, i) in item.notifications"
                        :key="i"
                        class="notification-badge"
                        :title="n.channel ? n.type + ': #' + n.channel : n.type"
                    >
                        {{ n.type }}{{ n.channel ? ': #' + truncateChannel(n.channel) : '' }}
                    </span>
                    <span v-if="!item.notifications || !item.notifications.length" class="grey--text">-</span>
                </div>
            </template>

            <template #item.actions="{ item }">
                <v-menu v-if="isFiring(item) || !item.suppressed || isReopenable(item)" offset-y>
                    <template v-slot:activator="{ attrs, on }">
                        <v-btn icon x-small class="ml-1" v-bind="attrs" v-on="on">
                            <v-icon small>mdi-dots-vertical</v-icon>
                        </v-btn>
                    </template>

                    <v-list dense>
                        <v-list-item v-if="isFiring(item)" @click="resolveSelected([item.id])" title="Acknowledge the alert; it will reopen if the condition recurs">
                            <v-icon small class="mr-1">mdi-check-circle-outline</v-icon> Resolve
                        </v-list-item>
                        <v-list-item v-if="!item.suppressed" @click="suppressSelected([item.id])" title="Permanently silence the alert; it will not reopen until manually reopened">
                            <v-icon small class="mr-1">mdi-bell-off-outline</v-icon> Suppress
                        </v-list-item>
                        <v-list-item v-if="isReopenable(item)" @click="reopenSelected([item.id])" title="Reopen the alert so it can fire again">
                            <v-icon small class="mr-1">mdi-restore</v-icon> Reopen
                        </v-list-item>
                    </v-list>
                </v-menu>
            </template>
        </v-data-table>

        <AlertDetail v-if="selectedAlertId" :key="selectedAlertId" :alert-id="selectedAlertId" @close="closeAlertDetail" @updated="get" />
    </div>
</template>

<script>
import AlertDetail from './AlertDetail.vue';

const statusDefs = {
    critical: { name: 'Critical', color: 'red lighten-1' },
    warning: { name: 'Warning', color: 'orange lighten-1' },
};

export default {
    components: { AlertDetail },

    data() {
        return {
            limit: Number(this.$route.query.limit) || 50,
            page: 1,
            sortBy: 'opened_at',
            sortDesc: true,
            alerts: [],
            serverTotal: 0,
            searchInput: '',
            search: '',
            showResolved: false,
            selected: [],
            searchTimeout: null,
        };
    },

    mounted() {
        this.showResolved = this.$route.query.show_resolved === '1';
        if (this.$route.query.search) {
            this.searchInput = this.$route.query.search;
            this.search = this.$route.query.search;
        }
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    computed: {
        items() {
            if (!this.alerts) {
                return [];
            }
            return this.alerts.map((a) => {
                let color = 'grey lighten-1'; // resolved/suppressed/manually resolved
                if (!a.resolved_at && !a.manually_resolved_at && !a.suppressed) {
                    color = a.severity === 'critical' ? 'red lighten-1' : 'orange lighten-1';
                }
                return {
                    ...a,
                    color,
                };
            });
        },
        statuses() {
            const contextAlerts = this.$api.context.alerts || {};
            return Object.keys(statusDefs).map((s) => {
                return {
                    ...statusDefs[s],
                    count: contextAlerts[s] || 0,
                };
            });
        },
        selectedFiring() {
            return this.selected.filter((id) => {
                const alert = this.items.find((a) => a.id === id);
                return alert && this.isFiring(alert);
            });
        },
        selectedSuppressible() {
            return this.selected.filter((id) => {
                const alert = this.items.find((a) => a.id === id);
                return alert && !alert.suppressed;
            });
        },
        selectedReopenable() {
            return this.selected.filter((id) => {
                const alert = this.items.find((a) => a.id === id);
                return alert && this.isReopenable(alert);
            });
        },
        allSelected() {
            return this.items.length > 0 && this.selected.length === this.items.length;
        },
        someSelected() {
            return this.selected.length > 0;
        },
        selectedAlertId() {
            return this.$route.query.alert;
        },
    },
    methods: {
        openAlert(id) {
            this.$router.push({ query: { ...this.$route.query, alert: id } }).catch((err) => err);
        },
        closeAlertDetail() {
            this.$router.replace({ query: { ...this.$route.query, alert: undefined } });
        },
        get() {
            this.$emit('loading', true);
            const offset = (this.page - 1) * this.limit;
            this.$api.getAlerts(
                {
                    limit: this.limit,
                    offset: offset,
                    includeResolved: this.showResolved,
                    search: this.search,
                    sortBy: this.sortBy,
                    sortDesc: this.sortDesc,
                },
                (data, error) => {
                    this.$emit('loading', false);
                    if (error) {
                        this.$emit('error', error);
                        return;
                    }
                    this.alerts = data?.alerts || [];
                    this.serverTotal = data?.total || 0;
                },
            );
        },
        onOptionsChange(options) {
            const newLimit = options.itemsPerPage;
            const newPage = options.page;
            const newSortBy = options.sortBy[0] || 'opened_at';
            const newSortDesc = options.sortDesc[0] !== false;

            const changed = this.limit !== newLimit || this.page !== newPage || this.sortBy !== newSortBy || this.sortDesc !== newSortDesc;

            if (changed) {
                this.limit = newLimit;
                this.page = newPage;
                this.sortBy = newSortBy;
                this.sortDesc = newSortDesc;

                this.$router.push({ query: { ...this.$route.query, limit: this.limit } }).catch((err) => err);
                this.get();
            }
        },
        changeShowResolved() {
            this.showResolved = !this.showResolved;
            this.page = 1;
            this.$router.push({ query: { ...this.$route.query, show_resolved: this.showResolved ? '1' : '0' } }).catch((err) => err);
            this.get();
        },
        debouncedSearch() {
            if (this.searchTimeout) {
                clearTimeout(this.searchTimeout);
            }
            this.searchTimeout = setTimeout(() => {
                this.search = this.searchInput;
                this.page = 1;
                this.get();
            }, 300);
        },
        clearSearch() {
            this.searchInput = '';
            this.search = '';
            this.page = 1;
            this.get();
        },
        toggleSelect(id) {
            const index = this.selected.indexOf(id);
            if (index >= 0) {
                this.selected.splice(index, 1);
            } else {
                this.selected.push(id);
            }
        },
        toggleSelectAll() {
            if (this.allSelected) {
                this.selected = [];
            } else {
                this.selected = this.items.map((a) => a.id);
            }
        },
        isFiring(alert) {
            return !alert.resolved_at && !alert.manually_resolved_at && !alert.suppressed;
        },
        isReopenable(alert) {
            return alert.manually_resolved_at || alert.suppressed;
        },
        resolveSelected(ids) {
            if (!Array.isArray(ids)) {
                ids = [...this.selectedFiring];
            }
            if (!ids.length) {
                return;
            }
            this.$api.resolveAlerts(ids, (data, error) => {
                if (error) {
                    this.$emit('error', error);
                    return;
                }
                this.selected = [];
                this.get();
            });
        },
        suppressSelected(ids) {
            if (!Array.isArray(ids)) {
                ids = [...this.selectedSuppressible];
            }
            if (!ids.length) {
                return;
            }
            this.$api.suppressAlerts(ids, (data, error) => {
                if (error) {
                    this.$emit('error', error);
                    return;
                }
                this.selected = [];
                this.get();
            });
        },
        reopenSelected(ids) {
            if (!Array.isArray(ids)) {
                ids = [...this.selectedReopenable];
            }
            if (!ids.length) {
                return;
            }
            this.$api.reopenAlerts(ids, (data, error) => {
                if (error) {
                    this.$emit('error', error);
                    return;
                }
                this.selected = [];
                this.get();
            });
        },
        codeSample(item) {
            const d = item.details && item.details.find((d) => d.code);
            return d ? d.value : '';
        },
        alertContextQuery(item) {
            return { alert: item.id };
        },
        truncateChannel(channel) {
            const maxLen = 15;
            if (channel.length <= maxLen) {
                return channel;
            }
            return channel.substring(0, maxLen) + '...';
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

.table:deep(td:has(.summary-cell)) {
    padding-left: 0 !important;
}

.summary-cell {
    display: flex;
    align-items: stretch;
    gap: 6px;
}
.severity-bar {
    width: 4px;
    flex-shrink: 0;
    border-radius: 2px;
}
.summary-content {
    min-width: 0;
    max-width: 700px;
    cursor: pointer;
}
.summary-link {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    cursor: pointer;
    color: inherit;
}
.summary-link:hover {
    text-decoration: underline;
}

.app-name {
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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

.notification-badge {
    font-size: 12px;
    padding: 1px 6px;
    border-radius: 3px;
    background-color: rgba(128, 128, 128, 0.15);
    white-space: nowrap;
}

.table:deep(.v-input--checkbox .v-icon) {
    color: rgba(0, 0, 0, 0.25) !important;
    font-size: 18px !important;
}
.table:deep(.v-input--checkbox.v-input--is-label-active .v-icon) {
    color: rgba(0, 0, 0, 0.7) !important;
}
.table:deep(.v-input--checkbox .v-input--selection-controls__ripple) {
    display: none;
}

.log-sample {
    font-family: monospace, monospace;
    font-size: 12px;
    background-color: var(--background-color-hi);
    filter: brightness(var(--brightness));
    border-radius: 3px;
    padding: 4px 6px;
    margin-top: 4px;
    max-height: 40px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
</style>
