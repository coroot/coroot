<template>
    <div>
        <div v-if="aggregatedResources.length === 0 && !loading" class="pa-3 text-center grey--text">No FluxCD resources found</div>

        <div v-else-if="!selectedResourceType">
            <div class="legend mb-3">
                <div v-for="status in legendStatuses" :key="status.name" class="item">
                    <div class="count" :style="{ backgroundColor: status.color }">{{ status.count }}</div>
                    <div class="label">{{ status.name }}</div>
                </div>
            </div>

            <v-simple-table dense class="table overview">
                <thead>
                    <tr>
                        <th>Resource Type</th>
                        <th>Count</th>
                        <th>Statuses</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="resource in aggregatedResources" :key="resource.type">
                        <td>
                            <router-link :to="getResourceTypeLink(resource.type)">
                                {{ resource.type }}
                            </router-link>
                        </td>
                        <td>{{ resource.total }}</td>
                        <td class="progress-cell">
                            <div class="bar">
                                <template v-for="status in statusOrder">
                                    <div
                                        v-if="resource.statusCounts[status] > 0"
                                        :key="status"
                                        :style="{
                                            width: getStatusPercentage(resource, status) + '%',
                                            backgroundColor: getStatusColorFromFlux(status),
                                        }"
                                    />
                                </template>
                            </div>
                        </td>
                    </tr>
                </tbody>
            </v-simple-table>
        </div>

        <!-- Detailed resource list view -->
        <div v-else>
            <h2 class="text-h6 font-weight-regular d-md-flex align-center mb-3">
                <v-btn icon @click="goBack"><v-icon>mdi-arrow-left</v-icon></v-btn>
                {{ selectedResourceType }}
            </h2>

            <div class="d-flex flex-column flex-sm-row flex-wrap flex-md-nowrap mb-4" style="gap: 12px">
                <div>
                    <v-text-field
                        v-model="search"
                        label="search"
                        clearable
                        dense
                        hide-details
                        prepend-inner-icon="mdi-magnify"
                        outlined
                        class="search"
                    />
                </div>

                <div>
                    <v-autocomplete
                        v-if="availableNamespaces.length > 0"
                        :items="namespacesForSelect"
                        v-model="selectedNamespaces"
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
                            <v-chip
                                small
                                label
                                close
                                close-icon="mdi-close"
                                @click:close="removeNamespace(item.value)"
                                color="primary"
                                class="namespace"
                            >
                                <span>{{ item.name }}</span>
                            </v-chip>
                        </template>
                    </v-autocomplete>
                </div>

                <div class="d-none d-sm-block flex-grow-1" />
            </div>

            <div class="legend mb-3">
                <div v-for="status in legendStatuses" :key="status.name" class="item">
                    <div class="count" :style="{ backgroundColor: status.color }">{{ status.count }}</div>
                    <div class="label">{{ status.name }}</div>
                </div>
            </div>

            <v-data-table
                sort-by="name"
                :sort-desc="false"
                must-sort
                dense
                class="table drill-down"
                mobile-breakpoint="0"
                :items-per-page="20"
                :items="filteredResources"
                item-key="id"
                :headers="getResourceColumns(selectedResourceType)"
                :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
            >
                <template #item.status="{ item }">
                    <div class="d-flex align-center">
                        <Led :status="mapFluxStatusToLed(item.status)" />
                        {{ item.status }}
                        <span v-if="item.reason">&nbsp;({{ item.reason }})</span>
                        <router-link :to="getEventsLink(item)" class="events-link ml-2">
                            <v-icon small>mdi-format-list-bulleted</v-icon>
                            <span>events</span>
                        </router-link>
                    </div>
                </template>
                <template #item.name="{ item }">
                    <div>{{ item.name }}</div>
                </template>
                <template #item.url="{ item }">
                    <div v-if="item.url">{{ item.url }}</div>
                    <div v-else>—</div>
                </template>
                <template #item.repository="{ item }">
                    <div v-if="item.repository_id" class="d-flex align-center">
                        <Led :status="mapFluxStatusToLed(getRepositoryStatusFromId(item.repository_id))" />
                        <router-link :to="getFluxResourceDrilldownLink(item.repository_id)">
                            {{ $utils.appId(item.repository_id).name }}
                        </router-link>
                    </div>
                    <div v-else>—</div>
                </template>
                <template #item.last_applied_revision="{ item }">
                    <div v-if="item.last_applied_revision">
                        <span :title="getRevisionTooltip(item)">{{ getShortHash(item.last_applied_revision) }}</span>
                    </div>
                    <div v-else>—</div>
                </template>
                <template #item.dependencies="{ item }">
                    <div v-if="item.dependencies && item.dependencies.length > 0">
                        <ul>
                            <li v-for="(dep, index) in getVisibleDependencies(item)" :key="dep">
                                <router-link v-if="isFluxResource(dep)" :to="getFluxResourceDrilldownLink(dep)">
                                    {{ formatResourceId(dep) }}
                                </router-link>
                                <span v-else>{{ formatResourceId(dep) }}</span>
                                <a
                                    v-if="
                                        index === getVisibleDependencies(item).length - 1 &&
                                        item.dependencies.length > 1 &&
                                        !isDependencyExpanded(item.id)
                                    "
                                    @click="toggleDependencyExpansion(item.id)"
                                    href="#"
                                >
                                    , +{{ item.dependencies.length - 1 }} more
                                </a>
                            </li>
                        </ul>
                        <a
                            v-if="item.dependencies.length > 1 && isDependencyExpanded(item.id)"
                            class="inventory-toggle"
                            @click="toggleDependencyExpansion(item.id)"
                        >
                            <span>Show less</span>
                        </a>
                    </div>
                    <div v-else>—</div>
                </template>
                <template #item.inventory_entries="{ item }">
                    <div v-if="item.inventory_entries && item.inventory_entries.length > 0">
                        <ul class="inventory-entries">
                            <li v-for="(entry, index) in getVisibleInventoryEntries(item)" :key="entry.id" class="inventory-entry">
                                <router-link
                                    v-if="entry.coroot_app_id && $utils.appId(entry.coroot_app_id).name"
                                    :to="getApplicationLink(entry.coroot_app_id)"
                                >
                                    {{ formatResourceId(entry.id) }}
                                </router-link>
                                <router-link v-else-if="isFluxResource(entry.id)" :to="getFluxResourceDrilldownLink(entry.id)">
                                    {{ formatResourceId(entry.id) }}
                                </router-link>
                                <span v-else>{{ formatResourceId(entry.id) }}</span>
                                <a
                                    v-if="
                                        index === getVisibleInventoryEntries(item).length - 1 &&
                                        item.inventory_entries.length > 1 &&
                                        !isInventoryExpanded(item.id)
                                    "
                                    @click="toggleInventoryExpansion(item.id)"
                                    href="#"
                                >
                                    , +{{ item.inventory_entries.length - 1 }} more
                                </a>
                            </li>
                        </ul>
                        <a
                            v-if="item.inventory_entries.length > 1 && isInventoryExpanded(item.id)"
                            class="inventory-toggle"
                            @click="toggleInventoryExpansion(item.id)"
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

export default {
    components: {
        Led,
    },
    data() {
        return {
            loading: false,
            error: '',
            resources: [],
            selectedResourceType: null,
            selectedNamespaces: [],
            search: '',
            searchDebounceTimer: null,
            statusOrder: ['Ready', 'Failed', 'Suspended', 'Unknown'],
            expandedInventories: {},
            expandedDependencies: {},
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
        this.stateFromUri();
    },

    beforeDestroy() {
        if (this.searchDebounceTimer) {
            clearTimeout(this.searchDebounceTimer);
        }
    },

    computed: {
        aggregatedResources() {
            return this.groupResourcesByType(this.resources);
        },
        legendStatuses() {
            const totals = this.createStatusCounts();
            const resources = this.selectedResourceType ? this.filteredResources : this.aggregatedResources;

            if (this.selectedResourceType) {
                resources.forEach((resource) => totals[resource.status]++);
            } else {
                resources.forEach((resource) => {
                    this.statusOrder.forEach((status) => (totals[status] += resource.statusCounts[status] || 0));
                });
            }

            return this.statusOrder.map((status) => ({
                name: status,
                count: totals[status],
                color: this.getStatusColorFromFlux(status),
            }));
        },
        availableNamespaces() {
            return this.selectedResourceType ? this.extractNamespaces(this.getResourcesOfType(this.selectedResourceType)) : [];
        },
        namespacesForSelect() {
            if (!this.selectedResourceType) return [];
            return this.createNamespaceOptions(this.getResourcesOfType(this.selectedResourceType));
        },
        filteredResources() {
            if (!this.selectedResourceType) return [];
            return this.applyFilters(this.getResourcesOfType(this.selectedResourceType));
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

        createStatusCounts() {
            return { Ready: 0, Failed: 0, Suspended: 0, Unknown: 0 };
        },

        groupResourcesByType(resources) {
            const groups = {};
            resources.forEach((resource) => {
                if (!groups[resource.type]) {
                    groups[resource.type] = {
                        type: resource.type,
                        total: 0,
                        statusCounts: this.createStatusCounts(),
                    };
                }
                groups[resource.type].total++;
                groups[resource.type].statusCounts[resource.status]++;
            });
            return Object.values(groups).sort((a, b) => a.type.localeCompare(b.type));
        },

        extractNamespaces(resources) {
            const namespaces = new Set();
            resources.forEach((resource) => {
                if (resource.namespace) namespaces.add(resource.namespace);
                if (resource.target_namespace && resource.target_namespace !== resource.namespace) {
                    namespaces.add(resource.target_namespace);
                }
            });
            return Array.from(namespaces).sort();
        },

        createNamespaceOptions(resources) {
            const map = {};
            [...this.selectedNamespaces, ...this.extractNamespaces(resources)].forEach((ns) => {
                if (!map[ns]) map[ns] = 0;
            });

            resources.forEach((resource) => {
                if (resource.namespace) map[resource.namespace]++;
                if (resource.target_namespace && resource.target_namespace !== resource.namespace) {
                    map[resource.target_namespace]++;
                }
            });

            return Object.keys(map)
                .map((ns) => ({
                    value: ns,
                    name: ns || '~empty',
                    count: map[ns],
                    text: `${ns || '~empty'} (${map[ns]})`,
                }))
                .sort((a, b) => a.value.localeCompare(b.value));
        },

        applyFilters(resources) {
            if (this.search) {
                const searchLower = this.search.toLowerCase();
                resources = resources.filter((resource) =>
                    ['name', 'namespace', 'target_namespace', 'url'].some((field) => resource[field]?.toLowerCase().includes(searchLower)),
                );
            }

            if (this.selectedNamespaces.length > 0) {
                const nsSet = new Set(this.selectedNamespaces);
                resources = resources.filter((resource) => nsSet.has(resource.namespace) || nsSet.has(resource.target_namespace));
            }

            return resources;
        },

        getStatusPercentage(resource, status) {
            return resource.total > 0 ? ((resource.statusCounts[status] || 0) / resource.total) * 100 : 0;
        },

        getResourceTypeLink(resourceType) {
            return {
                query: {
                    ...this.$route.query,
                    type: resourceType,
                },
            };
        },

        goBack() {
            this.updateQuery({ type: undefined, namespaces: undefined, search: undefined });
        },

        updateQuery(updates) {
            this.$router.replace({ query: { ...this.$route.query, ...updates } }).catch((err) => err);
        },

        getResourcesOfType(resourceType) {
            return this.resources.filter((resource) => resource.type === resourceType);
        },

        removeNamespace(ns) {
            const index = this.selectedNamespaces.indexOf(ns);
            if (index >= 0) this.selectedNamespaces.splice(index, 1);
        },
        getFluxResourceDrilldownLink(resourceId) {
            const resourceInfo = this.$utils.appId(resourceId);
            return {
                query: {
                    ...this.$route.query,
                    type: resourceInfo.kind,
                    namespaces: resourceInfo.ns ? [resourceInfo.ns] : undefined,
                    search: resourceInfo.name,
                },
            };
        },
        getRepositoryStatusFromId(repositoryId) {
            if (!repositoryId) return null;
            const repoResource = this.resources.find((r) => r.id === repositoryId);
            return repoResource ? repoResource.status : 'Unknown';
        },
        mapFluxStatusToLed(fluxStatus) {
            switch (fluxStatus) {
                case 'Ready':
                    return 'ok';
                case 'Failed':
                    return 'critical';
                case 'Suspended':
                    return 'warning';
                case 'Unknown':
                default:
                    return 'warning';
            }
        },
        getStatusColorFromFlux(status) {
            const colors = { Ready: '#4CAF50', Failed: '#F44336', Suspended: '#FF9800', Unknown: '#9E9E9E' };
            return colors[status] || '#9E9E9E';
        },
        getApplicationLink(applicationId) {
            return {
                name: 'overview',
                params: {
                    view: 'applications',
                    id: applicationId,
                },
                query: this.$utils.contextQuery(),
            };
        },
        formatResourceId(resourceId) {
            const parsed = this.$utils.appId(resourceId);
            return `${parsed.kind}:${parsed.name}`;
        },
        getVisibleItems(items, itemId, expandedState, shouldSort = false) {
            const processedItems = shouldSort ? this.sortInventoryEntries(items) : items;
            const isExpanded = !!expandedState[itemId];

            if (processedItems.length <= 1 || isExpanded) {
                return processedItems;
            }

            return processedItems.slice(0, 1);
        },
        sortInventoryEntries(entries) {
            return [...entries].sort((a, b) => {
                const aHasLink = a.coroot_app_id && this.$utils.appId(a.coroot_app_id).name;
                const bHasLink = b.coroot_app_id && this.$utils.appId(b.coroot_app_id).name;

                if (aHasLink && !bHasLink) return -1;
                if (!aHasLink && bHasLink) return 1;

                return this.formatResourceId(a.id).localeCompare(this.formatResourceId(b.id));
            });
        },
        getVisibleInventoryEntries(item) {
            return this.getVisibleItems(item.inventory_entries, item.id, this.expandedInventories, true);
        },
        getVisibleDependencies(item) {
            return this.getVisibleItems(item.dependencies, item.id, this.expandedDependencies, false);
        },
        isExpanded(itemId, expandedState) {
            return !!expandedState[itemId];
        },
        toggleExpansion(itemId, expandedState) {
            this.$set(expandedState, itemId, !expandedState[itemId]);
        },
        isInventoryExpanded(itemId) {
            return this.isExpanded(itemId, this.expandedInventories);
        },
        toggleInventoryExpansion(itemId) {
            this.toggleExpansion(itemId, this.expandedInventories);
        },
        isDependencyExpanded(itemId) {
            return this.isExpanded(itemId, this.expandedDependencies);
        },
        toggleDependencyExpansion(itemId) {
            this.toggleExpansion(itemId, this.expandedDependencies);
        },
        getEventsLink(item) {
            const query = {
                filters: [
                    { name: 'object.kind', op: '=', value: item.type },
                    { name: 'object.namespace', op: '=', value: item.namespace },
                    { name: 'object.name', op: '=', value: item.name },
                ],
            };

            return {
                name: 'overview',
                params: {
                    view: 'kubernetes',
                },
                query: {
                    ...this.$utils.contextQuery(),
                    query: JSON.stringify(query),
                },
            };
        },
        isFluxResource(entryId) {
            const fluxKinds = ['GitRepository', 'HelmRepository', 'OCIRepository', 'HelmChart', 'HelmRelease', 'Kustomization', 'ResourceSet'];
            const parsed = this.$utils.appId(entryId);
            return fluxKinds.includes(parsed.kind);
        },
        getShortHash(revision) {
            if (!revision) return '';
            const match = revision.match(/^(?:(.+?)[@/])?(?:sha\d+:)?([a-f0-9]{7,})$/i);
            if (match) {
                const [, branch, hash] = match;
                const shortHash = hash.substring(0, 7);
                return branch ? `${branch}@${shortHash}` : shortHash;
            }
            return revision.substring(0, 7);
        },
        getRevisionTooltip(item) {
            const parts = [];
            if (item.last_applied_revision) {
                parts.push(`Applied: ${item.last_applied_revision}`);
            }
            if (item.last_attempted_revision) {
                parts.push(`Attempted: ${item.last_attempted_revision}`);
            }
            return parts.join('\n');
        },
        getResourceColumns(resourceType) {
            const columnConfigs = {
                GitRepository: [
                    { value: 'name', text: 'Name', align: 'left', sortable: true },
                    { value: 'namespace', text: 'Namespace', align: 'left', sortable: true },
                    { value: 'status', text: 'Status', align: 'left', sortable: true },
                    { value: 'url', text: 'URL', align: 'left', sortable: false },
                    { value: 'interval', text: 'Interval', align: 'left', sortable: true },
                ],
                HelmRepository: [
                    { value: 'name', text: 'Name', align: 'left', sortable: true },
                    { value: 'namespace', text: 'Namespace', align: 'left', sortable: true },
                    { value: 'status', text: 'Status', align: 'left', sortable: true },
                    { value: 'url', text: 'URL', align: 'left', sortable: false },
                    { value: 'interval', text: 'Interval', align: 'left', sortable: true },
                ],
                OCIRepository: [
                    { value: 'name', text: 'Name', align: 'left', sortable: true },
                    { value: 'namespace', text: 'Namespace', align: 'left', sortable: true },
                    { value: 'status', text: 'Status', align: 'left', sortable: true },
                    { value: 'url', text: 'URL', align: 'left', sortable: false },
                    { value: 'interval', text: 'Interval', align: 'left', sortable: true },
                ],
                HelmChart: [
                    { value: 'name', text: 'Name', align: 'left', sortable: true },
                    { value: 'namespace', text: 'Namespace', align: 'left', sortable: true },
                    { value: 'status', text: 'Status', align: 'left', sortable: true },
                    { value: 'repository', text: 'Repository', align: 'left', sortable: true },
                    { value: 'chart', text: 'Chart', align: 'left', sortable: true },
                    { value: 'version', text: 'Version', align: 'left', sortable: true },
                    { value: 'interval', text: 'Interval', align: 'left', sortable: true },
                ],
                HelmRelease: [
                    { value: 'name', text: 'Name', align: 'left', sortable: true },
                    { value: 'namespace', text: 'Namespace', align: 'left', sortable: true },
                    { value: 'status', text: 'Status', align: 'left', sortable: true },
                    { value: 'repository', text: 'Repository', align: 'left', sortable: true },
                    { value: 'chart', text: 'Chart', align: 'left', sortable: true },
                    { value: 'version', text: 'Version', align: 'left', sortable: true },
                    { value: 'target_namespace', text: 'Target Namespace', align: 'left', sortable: true },
                    { value: 'interval', text: 'Interval', align: 'left', sortable: true },
                ],
                Kustomization: [
                    { value: 'name', text: 'Name', align: 'left', sortable: true },
                    { value: 'namespace', text: 'Namespace', align: 'left', sortable: true },
                    { value: 'status', text: 'Status', align: 'left', sortable: true },
                    { value: 'repository', text: 'Repository', align: 'left', sortable: true },
                    { value: 'last_applied_revision', text: 'Revision', align: 'left', sortable: false },
                    { value: 'dependencies', text: 'Dependencies', align: 'left', sortable: false },
                    { value: 'inventory_entries', text: 'Inventory Entries', align: 'left', sortable: false },
                    { value: 'target_namespace', text: 'Target Namespace', align: 'left', sortable: true },
                    { value: 'interval', text: 'Interval', align: 'left', sortable: true },
                ],
                ResourceSet: [
                    { value: 'name', text: 'Name', align: 'left', sortable: true },
                    { value: 'namespace', text: 'Namespace', align: 'left', sortable: true },
                    { value: 'status', text: 'Status', align: 'left', sortable: true },
                    { value: 'last_applied_revision', text: 'Revision', align: 'left', sortable: false },
                    { value: 'dependencies', text: 'Dependencies', align: 'left', sortable: false },
                    { value: 'inventory_entries', text: 'Inventory Entries', align: 'left', sortable: false },
                ],
            };
            return columnConfigs[resourceType] || [];
        },
        stateFromUri() {
            const query = this.$route.query;
            this.selectedResourceType = query.type || null;
            this.selectedNamespaces = Array.isArray(query.namespaces) ? query.namespaces : query.namespaces ? [query.namespaces] : [];
            this.search = query.search || '';
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
            this.updateQuery({
                namespaces: this.selectedNamespaces.length > 0 ? this.selectedNamespaces : undefined,
            });
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
.table.overview:deep(table) {
    max-width: 800px;
}

.table.drill-down:deep(table) {
    min-width: 500px;
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

.table:deep(.v-data-footer) {
    border-top: none;
    flex-wrap: nowrap;
}

.progress-cell {
    width: 50%;
}

.bar {
    display: flex;
    height: 16px;
    background-color: rgba(0, 0, 0, 0.1);
    filter: brightness(var(--brightness));
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

.search:deep(input) {
    width: 100px !important;
}

*:deep(.v-list-item) {
    font-size: 14px !important;
    padding: 0 8px !important;
}

*:deep(.v-list-item__action) {
    margin: 4px !important;
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
