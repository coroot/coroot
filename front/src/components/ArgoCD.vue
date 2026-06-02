<template>
    <div>
        <div v-if="resources.length === 0 && !loading" class="pa-3 text-center grey--text">No ArgoCD resources found</div>

        <div v-else>
            <div class="d-flex flex-column flex-sm-row flex-wrap flex-md-nowrap mb-4" style="gap: 12px">
                <v-text-field v-model="search" label="search" clearable dense hide-details prepend-inner-icon="mdi-magnify" outlined class="search" />
                <v-autocomplete
                    v-if="availableProjects.length > 0"
                    v-model="selectedProjects"
                    :items="projectsForSelect"
                    label="projects"
                    color="primary"
                    multiple
                    outlined
                    dense
                    chips
                    small-chips
                    deletable-chips
                    hide-details
                    class="projects"
                    :class="{ empty: !selectedProjects.length }"
                >
                    <template #selection="{ item }">
                        <v-chip small label close close-icon="mdi-close" @click:close="removeProject(item.value)" color="primary" class="project">
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
                :headers="appHeaders"
                :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
            >
                <template #item.name="{ item }">
                    <span :title="nameTitle(item)">{{ item.name }}</span>
                </template>
                <template #item.sync_status="{ item }">
                    <div class="d-flex align-center">
                        <Led :status="item.sync_level" />
                        {{ item.sync_status }}
                    </div>
                </template>
                <template #item.health_status="{ item }">
                    <div class="d-flex align-center">
                        <Led :status="item.health_level" />
                        {{ item.health_status }}
                    </div>
                </template>
                <template #item.operation_phase="{ item }">
                    <div class="d-flex align-center">
                        <template v-if="item.operation_phase">
                            <Led :status="item.operation_level" />
                            <span>{{ item.operation_phase }}</span>
                            <span
                                v-if="item.operation_finished_at"
                                class="op-time"
                                :title="$format.date(item.operation_finished_at * 1000, '{YYYY}-{MM}-{DD} {HH}:{mm}:{ss}')"
                            >
                                · {{ $format.timeSinceNow(item.operation_finished_at * 1000) }} ago
                            </span>
                        </template>
                        <span v-else>—</span>
                        <router-link :to="eventsLink(item)" class="events-link ml-2">
                            <v-icon small>mdi-format-list-bulleted</v-icon>
                            <span>events</span>
                        </router-link>
                    </div>
                </template>
                <template #item.source="{ item }">
                    <div class="d-flex align-center" :title="item.repo" style="max-width: 28ch">
                        <v-icon v-if="sourceTypeIcon(item)" size="13" class="mr-1" color="inherit">{{ sourceTypeIcon(item) }}</v-icon>
                        <span class="text-truncate">{{ sourceSubtitle(item) || $format.repo(item.repo) }}</span>
                    </div>
                </template>
                <template #item.resources="{ item }">
                    <div v-if="item.resources && item.resources.length > 0">
                        <ul class="resources">
                            <li v-for="(entry, index) in getVisibleResources(item)" :key="entry.id" class="resource">
                                <router-link v-if="entry.coroot_app_id && $utils.appId(entry.coroot_app_id).name" :to="appLink(entry.coroot_app_id)">
                                    {{ formatResourceId(entry.id) }}
                                </router-link>
                                <span v-else>{{ formatResourceId(entry.id) }}</span>
                                <span v-if="entry.issue" class="resource-issue text--secondary"> <Led status="warning" />{{ entry.issue }} </span>
                                <a
                                    v-if="index === getVisibleResources(item).length - 1 && item.resources.length > 1 && !isResourceExpanded(item.id)"
                                    @click="toggleResourceExpansion(item.id)"
                                    href="#"
                                >
                                    , +{{ item.resources.length - 1 }} more
                                </a>
                            </li>
                        </ul>
                        <a
                            v-if="item.resources.length > 1 && isResourceExpanded(item.id)"
                            class="resources-toggle"
                            @click="toggleResourceExpansion(item.id)"
                        >
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

const FACETS = [
    { key: 'sync', label: 'Sync', statusField: 'sync_status', levelField: 'sync_level' },
    { key: 'health', label: 'Health', statusField: 'health_status', levelField: 'health_level' },
];

export default {
    components: { Led, StatusFacets },
    data() {
        return {
            loading: false,
            error: '',
            resources: [],
            selectedProjects: [],
            selectedStatus: { sync: null, health: null },
            search: '',
            searchDebounceTimer: null,
            expandedResources: {},
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
        apps() {
            return this.resources;
        },
        availableProjects() {
            const s = new Set();
            this.apps.forEach((a) => a.project && s.add(a.project));
            return Array.from(s).sort();
        },
        projectsForSelect() {
            const counts = {};
            this.apps.forEach((a) => {
                if (a.project) counts[a.project] = (counts[a.project] || 0) + 1;
            });
            this.selectedProjects.forEach((p) => {
                if (counts[p] === undefined) counts[p] = 0;
            });
            return Object.keys(counts)
                .sort()
                .map((p) => ({ value: p, text: `${p} (${counts[p]})` }));
        },
        baseApps() {
            let items = this.apps;
            if (this.selectedProjects.length > 0) {
                const set = new Set(this.selectedProjects);
                items = items.filter((a) => set.has(a.project));
            }
            if (this.search) {
                const q = this.search.toLowerCase();
                items = items.filter((a) =>
                    ['name', 'project', 'repo', 'path', 'cluster', 'sync_status', 'health_status', 'operation_phase'].some((f) =>
                        (a[f] || '').toLowerCase().includes(q),
                    ),
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
        appHeaders() {
            const cols = [
                { value: 'name', text: 'Name', sortable: true },
                { value: 'project', text: 'Project', sortable: true },
                { value: 'cluster', text: 'Cluster', sortable: true },
                { value: 'sync_status', text: 'Sync', sortable: true },
                { value: 'health_status', text: 'Health', sortable: true },
                { value: 'operation_phase', text: 'Last sync', sortable: true },
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
            this.$api.getOverview('argocd', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.resources = data.argocd || [];
            });
        },
        updateQuery(updates) {
            this.$router.replace({ query: { ...this.$route.query, ...updates } }).catch((err) => err);
        },
        stateFromUri() {
            const q = this.$route.query;
            this.selectedProjects = Array.isArray(q.projects) ? q.projects : q.projects ? [q.projects] : [];
            this.selectedStatus = { sync: q.sync || null, health: q.health || null };
            this.search = q.search || '';
        },
        sourceSubtitle(item) {
            const detail = item.chart || item.path;
            if (item.source_type && detail) return `${item.source_type} · ${detail}`;
            return item.source_type || detail || '';
        },
        nameTitle(item) {
            return item.namespace ? `namespace: ${item.namespace}` : '';
        },
        sourceTypeIcon(item) {
            if (item.source_type === 'Helm') return 'mdi-ship-wheel';
            if (item.source_type === 'Kustomize' || item.source_type === 'Directory') return 'mdi-source-branch';
            return '';
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
                    { name: 'object.kind', op: '=', value: 'Application' },
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
        sortedResources(item) {
            return [...item.resources].sort((a, b) => {
                if (!!a.issue !== !!b.issue) return a.issue ? -1 : 1;
                const aL = a.coroot_app_id && this.$utils.appId(a.coroot_app_id).name;
                const bL = b.coroot_app_id && this.$utils.appId(b.coroot_app_id).name;
                if (aL && !bL) return -1;
                if (!aL && bL) return 1;
                return this.formatResourceId(a.id).localeCompare(this.formatResourceId(b.id));
            });
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
        getVisibleResources(item) {
            const all = this.sortedResources(item);
            return this.isResourceExpanded(item.id) || all.length <= 1 ? all : all.slice(0, 1);
        },
        isResourceExpanded(id) {
            return !!this.expandedResources[id];
        },
        toggleResourceExpansion(id) {
            this.$set(this.expandedResources, id, !this.expandedResources[id]);
        },
        removeProject(p) {
            const i = this.selectedProjects.indexOf(p);
            if (i >= 0) this.selectedProjects.splice(i, 1);
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
        selectedProjects() {
            this.updateQuery({ projects: this.selectedProjects.length > 0 ? this.selectedProjects : undefined });
        },
        selectedStatus: {
            handler() {
                this.updateQuery({
                    sync: this.selectedStatus.sync || undefined,
                    health: this.selectedStatus.health || undefined,
                });
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
.projects:deep(input) {
    width: 0 !important;
}
.projects.empty:deep(input) {
    width: 100px !important;
}
.project {
    margin: 4px 4px 0 0 !important;
    padding: 0 8px !important;
}
.project span {
    max-width: 20ch;
    overflow: hidden;
    text-overflow: ellipsis;
}
.project:deep(.v-icon) {
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
.resource-issue {
    font-size: 12px;
    margin-left: 4px;
    white-space: nowrap;
}
.op-time {
    margin-left: 4px;
    font-size: 12px;
    color: rgba(0, 0, 0, 0.6);
}
.events-link {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 6px;
    border-radius: 4px;
    background-color: rgba(25, 118, 210, 0.1);
    color: #1976d2;
    text-decoration: none;
    font-size: 12px;
    transition: background-color 0.2s;
}
.events-link:hover {
    background-color: rgba(25, 118, 210, 0.2);
    text-decoration: none;
}
</style>
