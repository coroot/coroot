<template>
    <div>
        <div v-if="resources.length === 0 && !loading" class="pa-3 text-center grey--text">No FluxCD resources found</div>

        <div v-else>
            <div class="d-flex flex-column flex-sm-row flex-wrap flex-md-nowrap mb-4" style="gap: 12px">
                <v-text-field v-model="search" label="search" clearable dense hide-details prepend-inner-icon="mdi-magnify" outlined class="search" />
                <v-autocomplete
                    v-if="availableNamespaces.length > 0"
                    v-model="selectedNamespaces"
                    :items="namespacesForSelect"
                    label="namespaces"
                    color="primary"
                    multiple
                    outlined
                    dense
                    chips
                    small-chips
                    deletable-chips
                    hide-details
                    class="namespaces"
                    :class="{ empty: !selectedNamespaces.length }"
                >
                    <template #selection="{ item }">
                        <v-chip small label close close-icon="mdi-close" @click:close="removeNamespace(item.value)" color="primary" class="namespace">
                            <span :title="item.text">{{ item.value }}</span>
                        </v-chip>
                    </template>
                </v-autocomplete>
                <div class="d-none d-sm-block flex-grow-1" />
            </div>

            <StatusFacets :facets="facets" :selected="selectedStatus" @toggle="toggleStatus" />

            <v-data-table
                sort-by="name"
                must-sort
                dense
                class="table drill-down"
                mobile-breakpoint="0"
                :items-per-page="20"
                :items="filteredApps"
                item-key="id"
                :headers="headers"
                :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
            >
                <template #item.name="{ item }">
                    <div class="d-flex align-center" :title="nameTitle(item)">
                        <v-icon size="14" class="mr-1" color="inherit">{{ kindIcon(item.type) }}</v-icon>
                        {{ item.name }}
                    </div>
                </template>
                <template #item.status="{ item }">
                    <div class="d-flex align-center">
                        <Led :status="item.level" />
                        {{ item.status }}
                        <span v-if="item.reason" class="text--secondary ml-1">({{ item.reason }})</span>
                        <router-link :to="eventsLink(item)" class="events-link ml-2">
                            <v-icon small>mdi-format-list-bulleted</v-icon>
                            <span>events</span>
                        </router-link>
                    </div>
                </template>
                <template #item.source="{ item }">
                    <div v-if="source(item)">
                        <div class="d-flex align-center">
                            <Led :status="source(item).level" />
                            <span>{{ source(item).status }}</span>
                            <v-icon size="13" class="ml-2 mr-1 text--secondary" color="inherit">{{ sourceIcon(item) }}</v-icon>
                            <span class="text--secondary text-truncate" style="max-width: 20ch" :title="sourceUrl(item)">{{
                                $format.repo(sourceUrl(item))
                            }}</span>
                        </div>
                        <div
                            v-if="sourceIssue(item)"
                            class="text--secondary text-truncate"
                            style="font-size: 12px; max-width: 24ch"
                            :title="sourceIssue(item).reason || sourceIssue(item).status"
                        >
                            {{ sourceIssue(item).reason || sourceIssue(item).status }}
                        </div>
                    </div>
                    <div v-else>—</div>
                </template>
                <template #item.resources="{ item }">
                    <div v-if="item.inventory_entries && item.inventory_entries.length > 0">
                        <ul class="resources">
                            <li v-for="(entry, index) in getVisibleEntries(item)" :key="entry.id" class="resource">
                                <router-link v-if="entry.coroot_app_id && $utils.appId(entry.coroot_app_id).name" :to="appLink(entry.coroot_app_id)">
                                    {{ formatResourceId(entry.id) }}
                                </router-link>
                                <span v-else>{{ formatResourceId(entry.id) }}</span>
                                <a
                                    v-if="index === getVisibleEntries(item).length - 1 && item.inventory_entries.length > 1 && !isExpanded(item.id)"
                                    @click="toggleExpansion(item.id)"
                                    href="#"
                                >
                                    , +{{ item.inventory_entries.length - 1 }} more
                                </a>
                            </li>
                        </ul>
                        <a v-if="item.inventory_entries.length > 1 && isExpanded(item.id)" class="resources-toggle" @click="toggleExpansion(item.id)">
                            <span>Show less</span>
                        </a>
                    </div>
                    <div v-else>—</div>
                </template>
            </v-data-table>
        </div>
    </div>
</template>

<script>
import Led from './Led';
import StatusFacets from './StatusFacets';

const APP_KINDS = ['Kustomization', 'HelmRelease', 'ResourceSet'];
const FACETS = [{ key: 'status', label: 'Status', statusField: 'status', levelField: 'level' }];

export default {
    components: { Led, StatusFacets },
    data() {
        return {
            loading: false,
            error: '',
            resources: [],
            selectedNamespaces: [],
            selectedStatus: { status: null },
            search: '',
            searchDebounceTimer: null,
            expandedEntries: {},
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
        this.stateFromUri();
    },
    beforeDestroy() {
        if (this.searchDebounceTimer) clearTimeout(this.searchDebounceTimer);
    },

    computed: {
        resourcesById() {
            const m = {};
            this.resources.forEach((r) => (m[r.id] = r));
            return m;
        },
        apps() {
            return this.resources.filter((r) => APP_KINDS.includes(r.type));
        },
        availableNamespaces() {
            const s = new Set();
            this.apps.forEach((a) => a.namespace && s.add(a.namespace));
            return Array.from(s).sort();
        },
        namespacesForSelect() {
            const counts = {};
            this.apps.forEach((a) => {
                if (a.namespace) counts[a.namespace] = (counts[a.namespace] || 0) + 1;
            });
            this.selectedNamespaces.forEach((ns) => {
                if (counts[ns] === undefined) counts[ns] = 0;
            });
            return Object.keys(counts)
                .sort()
                .map((ns) => ({ value: ns, text: `${ns} (${counts[ns]})` }));
        },
        baseApps() {
            let items = this.apps;
            if (this.selectedNamespaces.length > 0) {
                const set = new Set(this.selectedNamespaces);
                items = items.filter((a) => set.has(a.namespace));
            }
            if (this.search) {
                const q = this.search.toLowerCase();
                items = items.filter((a) =>
                    [a.name, a.type, a.namespace, a.target_namespace, this.sourceUrl(a)].some((f) => (f || '').toLowerCase().includes(q)),
                );
            }
            return items;
        },
        facets() {
            return FACETS.map((f) => {
                const counted = this.applyStatus(this.baseApps, f.key);
                const counts = {};
                counted.forEach((a) => {
                    const v = a[f.statusField] || 'Unknown';
                    counts[v] = (counts[v] || 0) + 1;
                });
                const levels = {};
                this.baseApps.forEach((a) => {
                    const v = a[f.statusField] || 'Unknown';
                    if (!(v in levels)) levels[v] = a[f.levelField] || 'unknown';
                });
                const options = Object.keys(levels).map((v) => ({ value: v, count: counts[v] || 0, level: levels[v] }));
                return { key: f.key, label: f.label, options };
            });
        },
        filteredApps() {
            return this.applyStatus(this.baseApps, null);
        },
        headers() {
            const cols = [
                { value: 'name', text: 'Name', sortable: true },
                { value: 'cluster', text: 'Cluster', sortable: true },
                { value: 'status', text: 'Status', sortable: true },
                { value: 'source', text: 'Source', sortable: false },
                { value: 'resources', text: 'Resources', sortable: false },
            ];
            return this.$api.context && this.$api.context.multicluster ? cols : cols.filter((c) => c.value !== 'cluster');
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getOverview('fluxcd', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.resources = data.fluxcd || [];
            });
        },
        kindIcon(type) {
            return type === 'HelmRelease' ? 'mdi-ship-wheel' : 'mdi-source-branch';
        },
        nameTitle(item) {
            const parts = [`kind: ${item.type}`];
            if (item.namespace) parts.push(`namespace: ${item.namespace}`);
            if (item.target_namespace) parts.push(`target namespace: ${item.target_namespace}`);
            return parts.join('\n');
        },
        source(item) {
            return item.repository_id ? this.resourcesById[item.repository_id] : null;
        },
        sourceUrl(item) {
            const s = this.source(item);
            return s ? s.url : '';
        },
        sourceIcon(item) {
            const s = this.source(item);
            if (!s) return '';
            switch (s.type) {
                case 'HelmRepository':
                    return 'mdi-ship-wheel';
                case 'OCIRepository':
                    return 'mdi-package-variant';
                default: // GitRepository
                    return 'mdi-source-branch';
            }
        },
        sourceIssue(item) {
            const s = this.source(item);
            return s && s.level === 'warning' ? s : null;
        },
        applyStatus(items, exceptKey) {
            FACETS.forEach((f) => {
                if (f.key === exceptKey) return;
                const v = this.selectedStatus[f.key];
                if (!v) return;
                items = items.filter((a) => (a[f.statusField] || 'Unknown') === v);
            });
            return items;
        },
        toggleStatus({ key, value }) {
            this.selectedStatus = {
                ...this.selectedStatus,
                [key]: this.selectedStatus[key] === value ? null : value,
            };
        },
        formatResourceId(rid) {
            const p = this.$utils.appId(rid);
            return `${p.kind}:${p.name}`;
        },
        appLink(applicationId) {
            return {
                name: 'overview',
                params: { view: 'applications', id: applicationId },
                query: this.$utils.contextQuery(),
            };
        },
        eventsLink(item) {
            const query = {
                filters: [
                    { name: 'object.kind', op: '=', value: item.type },
                    { name: 'object.namespace', op: '=', value: item.namespace },
                    { name: 'object.name', op: '=', value: item.name },
                ],
            };
            return {
                name: 'overview',
                params: { view: 'kubernetes' },
                query: { ...this.$utils.contextQuery(), query: JSON.stringify(query) },
            };
        },
        sortedEntries(item) {
            return [...item.inventory_entries].sort((a, b) => {
                const aL = a.coroot_app_id && this.$utils.appId(a.coroot_app_id).name;
                const bL = b.coroot_app_id && this.$utils.appId(b.coroot_app_id).name;
                if (aL && !bL) return -1;
                if (!aL && bL) return 1;
                return this.formatResourceId(a.id).localeCompare(this.formatResourceId(b.id));
            });
        },
        getVisibleEntries(item) {
            const all = this.sortedEntries(item);
            return this.isExpanded(item.id) || all.length <= 1 ? all : all.slice(0, 1);
        },
        isExpanded(id) {
            return !!this.expandedEntries[id];
        },
        toggleExpansion(id) {
            this.$set(this.expandedEntries, id, !this.expandedEntries[id]);
        },
        removeNamespace(ns) {
            const i = this.selectedNamespaces.indexOf(ns);
            if (i >= 0) this.selectedNamespaces.splice(i, 1);
        },
        updateQuery(updates) {
            this.$router.replace({ query: { ...this.$route.query, ...updates } }).catch((err) => err);
        },
        stateFromUri() {
            const q = this.$route.query;
            this.selectedNamespaces = Array.isArray(q.namespaces) ? q.namespaces : q.namespaces ? [q.namespaces] : [];
            this.selectedStatus = { status: q.status || null };
            this.search = q.search || '';
        },
    },

    watch: {
        loading(val) {
            this.$emit('loading', val);
        },
        error(val) {
            this.$emit('error', val);
        },
        '$route.query': {
            handler() {
                this.stateFromUri();
            },
            immediate: false,
        },
        selectedNamespaces() {
            this.updateQuery({ namespaces: this.selectedNamespaces.length > 0 ? this.selectedNamespaces : undefined });
        },
        selectedStatus: {
            handler() {
                this.updateQuery({ status: this.selectedStatus.status || undefined });
            },
            deep: true,
        },
        search() {
            if (this.searchDebounceTimer) clearTimeout(this.searchDebounceTimer);
            this.searchDebounceTimer = setTimeout(() => {
                this.updateQuery({ search: this.search || undefined });
            }, 500);
        },
    },
};
</script>

<style scoped>
.table.drill-down:deep(table) {
    min-width: 600px;
}
.table:deep(tr:hover) {
    background-color: unset !important;
}
.table:deep(th) {
    white-space: nowrap;
}
.table:deep(th),
.table:deep(td) {
    padding: 4px 8px !important;
}
.table:deep(td) {
    vertical-align: top;
}
.table:deep(.v-data-footer) {
    border-top: none;
    flex-wrap: nowrap;
}
.search:deep(input) {
    width: 100px !important;
}
.namespaces:deep(input) {
    width: 0 !important;
}
.namespaces.empty:deep(input) {
    width: 100px !important;
}
.namespace {
    margin: 4px 4px 0 0 !important;
    padding: 0 8px !important;
}
.namespace span {
    max-width: 20ch;
    overflow: hidden;
    text-overflow: ellipsis;
}
.namespace:deep(.v-icon) {
    font-size: 16px !important;
}
.resources {
    list-style: none;
    padding: 0;
    margin: 0;
}
.resource {
    padding: 1px 0;
}
.resources-toggle {
    font-size: 12px;
}
.events-link {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 6px;
    border-radius: 4px;
    background-color: rgba(25, 118, 210, 0.1);
    color: var(--brand-primary, #5d54a4);
    text-decoration: none;
    font-size: 12px;
    transition: background-color 0.2s;
}
.events-link:hover {
    background-color: rgba(25, 118, 210, 0.2);
    text-decoration: none;
}
</style>
