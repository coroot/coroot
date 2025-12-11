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
                <span v-if="item.gpus">{{ item.gpus }}</span>
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

export default {
    components: { Led, Views, AgentInstallation },

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
</style>
