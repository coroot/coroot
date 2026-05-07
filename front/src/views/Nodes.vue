<template>
    <Views :loading="loading" :error="error">
        <div class="d-flex">
            <v-text-field v-model="search" label="search" clearable dense hide-details prepend-inner-icon="mdi-magnify" outlined class="search" />
        </div>
        <v-data-table
            sort-by="name"
            must-sort
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="100"
            :items="nodes_"
            item-key="item"
            no-data-text="No nodes found"
            :headers="headers"
            :footer-props="{ itemsPerPageOptions: [5, 10, 20, 50, 100, -1] }"
        >
            <template #item.name="{ item }">
                <router-link
                    :to="{ name: 'overview', params: { view: 'nodes', id: item.cluster_id + ':' + item.name, query: $utils.contextQuery() } }"
                    class="truncated"
                    style="max-width: 30ch"
                >
                    {{ item.name }}
                </router-link>
            </template>

            <template #item.cluster="{ item }">
                <span class="truncated grey--text" style="max-width: 20ch">{{ item.cluster_name }}</span>
            </template>

            <template #item.os="{ item }">
                <AppIcon :icon="item.os" style="vertical-align: middle" />
                <span v-if="item.kernel_version" class="caption grey--text ml-1">{{ item.kernel_version }}</span>
            </template>

            <template #item.status="{ item }">
                <Led :status="item.status.status" />
                {{ item.status.message }}
                <span v-if="item.uptime_ms" class="caption grey--text"> ({{ $format.durationPretty(item.uptime_ms) }})</span>
            </template>

            <template #item.compute="{ item }">
                <template v-if="item.compute">
                    {{ item.compute }}
                    <span v-if="item.instance_type" class="caption grey--text truncated"> ({{ item.instance_type }})</span>
                </template>
            </template>

            <template #item.availability_zone="{ item }">
                <span class="truncated">{{ item.availability_zone }}</span>
                <span v-if="item.cloud_provider" class="caption grey--text"> ({{ item.cloud_provider }})</span>
            </template>

            <template #item.gpus="{ item }">
                <div v-if="item.gpus" class="gpu-cell">
                    <template v-if="item.gpu_stats && item.gpu_stats.length">
                        <div v-for="gpu in item.gpu_stats" :key="gpu.uuid" class="gpu-row">
                            <div class="d-flex align-center">
                                <span class="truncated gpu-name">{{ gpu.name || gpu.uuid }}</span>
                                <span v-if="gpuMemory(gpu)" class="caption grey--text ml-1 text-no-wrap">{{ gpuMemory(gpu) }}</span>
                            </div>
                            <div v-for="metric in gpuMetricRows(gpu)" :key="metric.label" class="gpu-meter">
                                <span class="gpu-meter-label">{{ metric.label }}</span>
                                <v-progress-linear
                                    :background-color="metric.backgroundColor"
                                    height="14"
                                    :color="metric.color"
                                    :value="clampPercent(metricValue(metric))"
                                >
                                    <span style="font-size: 11px">{{ formatPercent(metricValue(metric)) }}</span>
                                </v-progress-linear>
                                <span v-if="hasMetric(metric.peak)" class="caption grey--text gpu-peak">pk {{ formatPercent(metric.peak) }}</span>
                            </div>
                        </div>
                    </template>
                    <span v-else>{{ gpuCountText(item.gpus) }}</span>
                </div>
                <span v-else>-</span>
            </template>

            <template #item.ips="{ item }">
                <v-menu v-if="item.ips.length > 1" offset-y tile>
                    <template #activator="{ on }">
                        <span v-on="on" class="text-no-wrap"> {{ item.ips[0] }}, ...</span>
                    </template>
                    <v-list dense>
                        <v-list-item v-for="v in item.ips" style="font-size: 14px; min-height: 32px">
                            <v-list-item-title>{{ v }}</v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
                <span v-else>{{ item.ips[0] }}</span>
            </template>

            <template #item.cpu_percent="{ item }">
                <v-progress-linear
                    v-if="item.cpu_percent"
                    background-color="blue lighten-3"
                    height="16"
                    color="blue lighten-1"
                    :value="item.cpu_percent"
                    style="min-width: 64px"
                >
                    <span style="font-size: 14px">{{ item.cpu_percent }}%</span>
                </v-progress-linear>
            </template>
            <template #item.memory_percent="{ item }">
                <v-progress-linear
                    v-if="item.memory_percent"
                    background-color="purple lighten-3"
                    height="16"
                    color="purple lighten-1"
                    :value="item.memory_percent"
                    style="min-width: 64px"
                >
                    <span style="font-size: 14px">{{ item.memory_percent }}%</span>
                </v-progress-linear>
            </template>
            <template #item.total_network_bandwidth="{ item }">
                <template v-if="item.network_bandwidth">
                    <span class="text-no-wrap">
                        <v-icon small color="green">mdi-arrow-down-thick</v-icon>{{ $format.formatBandwidth(item.network_bandwidth.rx) }}
                    </span>
                    <span class="text-no-wrap">
                        <v-icon small color="blue">mdi-arrow-up-thick</v-icon>{{ $format.formatBandwidth(item.network_bandwidth.tx) }}
                    </span>
                </template>
                {{ item.network }}
            </template>
        </v-data-table>

        <div class="mt-4">
            <AgentInstallation color="primary">Add nodes</AgentInstallation>
        </div>
    </Views>
</template>

<script>
import Views from '@/views/Views.vue';
import AgentInstallation from '@/views/AgentInstallation.vue';
import Led from '@/components/Led.vue';
import AppIcon from '@/components/AppIcon.vue';

export default {
    components: { Led, Views, AgentInstallation, AppIcon },

    data() {
        return {
            nodes: [],
            loading: false,
            error: '',
            search: '',
        };
    },

    computed: {
        headers() {
            let headers = [
                { value: 'name', text: 'Name', align: 'left' },
                { value: 'status', text: 'Status', align: 'left' },
                { value: 'compute', text: 'Compute', align: 'left', sortable: false },
                { value: 'availability_zone', text: 'Availability zone', align: 'left' },
                { value: 'ips', text: 'IP', align: 'left', sortable: false },
                { value: 'cpu_percent', text: 'CPU', align: 'left' },
                { value: 'memory_percent', text: 'Memory', align: 'left' },
                { value: 'gpus', text: 'GPU', align: 'left' },
                { value: 'total_network_bandwidth', text: 'Network', align: 'left' },
                { value: 'os', text: 'OS', align: 'left' },
                { value: 'cluster', text: 'Cluster', align: 'left' },
            ];
            if (!this.$api.context.multicluster) {
                return headers.filter((h) => h.value !== 'cluster');
            }
            return headers;
        },
        nodes_() {
            if (!this.search) {
                return this.nodes;
            }
            return this.nodes.filter((n) =>
                (n.name + ' ' + n.availability_zone + ' ' + n.cluster_name + ' ' + n.ips.join(' ')).toLowerCase().includes(this.search.toLowerCase()),
            );
        },
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getOverview('nodes', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.nodes = data.nodes;
            });
        },
        hasMetric(value) {
            return value !== null && value !== undefined;
        },
        metricValue(metric) {
            return this.hasMetric(metric.avg) ? metric.avg : metric.peak;
        },
        clampPercent(value) {
            if (!this.hasMetric(value)) {
                return 0;
            }
            return Math.max(0, Math.min(100, value));
        },
        formatPercent(value) {
            if (!this.hasMetric(value)) {
                return '-';
            }
            return this.$format.percent(value) + '%';
        },
        gpuCountText(count) {
            return count === 1 ? '1 GPU' : `${count} GPUs`;
        },
        gpuMemory(gpu) {
            if (this.hasMetric(gpu.used_memory_bytes) && this.hasMetric(gpu.total_memory_bytes)) {
                return `${this.$format.formatBytes(gpu.used_memory_bytes)} / ${this.$format.formatBytes(gpu.total_memory_bytes)}`;
            }
            if (this.hasMetric(gpu.total_memory_bytes)) {
                return this.$format.formatBytes(gpu.total_memory_bytes);
            }
            return '';
        },
        gpuMetricRows(gpu) {
            return [
                {
                    label: 'GPU',
                    avg: gpu.usage_average_percent,
                    peak: gpu.usage_peak_percent,
                    color: 'green lighten-1',
                    backgroundColor: 'green lighten-4',
                },
                {
                    label: 'Mem',
                    avg: gpu.memory_usage_average_percent,
                    peak: gpu.memory_usage_peak_percent,
                    color: 'purple lighten-1',
                    backgroundColor: 'purple lighten-4',
                },
                {
                    label: 'SM',
                    avg: gpu.compute_occupancy_average_percent,
                    peak: gpu.compute_occupancy_peak_percent,
                    color: 'orange lighten-1',
                    backgroundColor: 'orange lighten-4',
                },
            ].filter((metric) => this.hasMetric(metric.avg) || this.hasMetric(metric.peak));
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
.search {
    max-width: 200px !important;
}
.truncated {
    max-width: 15ch;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    display: inline-block;
    vertical-align: bottom;
}
.gpu-cell {
    min-width: 220px;
}
.gpu-row + .gpu-row {
    border-top: 1px solid rgba(0, 0, 0, 0.08);
    margin-top: 4px;
    padding-top: 4px;
}
.gpu-name {
    max-width: 20ch;
}
.gpu-meter {
    align-items: center;
    display: grid;
    gap: 4px;
    grid-template-columns: 28px minmax(72px, 1fr) 44px;
    height: 16px;
}
.gpu-meter-label {
    color: rgba(0, 0, 0, 0.6);
    font-size: 11px;
    line-height: 1;
}
.gpu-peak {
    font-size: 11px !important;
    white-space: nowrap;
}
</style>
