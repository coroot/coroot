<template>
    <div>
        <h1 class="text-h5 my-5">
            Overview
            <v-progress-circular v-if="loading" indeterminate color="green" />
        </h1>

        <v-tabs height="40" show-arrows slider-size="2" class="mb-3">
            <template v-for="(name, id) in views">
                <v-tab
                    v-if="name"
                    :to="{ params: { view: id === 'health' ? undefined : id }, query: $utils.contextQuery() }"
                    exact-path
                    :class="{ disabled: loading }"
                >
                    {{ name }}
                </v-tab>
            </template>
        </v-tabs>

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <template v-if="!view">
            <Health v-if="health" :applications="health" />
            <NoData v-else-if="!loading && !error" />
        </template>

        <template v-else-if="view === 'map'">
            <ServiceMap v-if="map" :applications="map" :categories="categories" />
            <NoData v-else-if="!loading && !error" />
        </template>

        <template v-else-if="view === 'nodes'">
            <template v-if="nodes && nodes.rows">
                <Table v-if="nodes && nodes.rows" :header="nodes.header" :rows="nodes.rows" />
                <div class="mt-4">
                    <AgentInstallation color="primary">Add nodes</AgentInstallation>
                </div>
            </template>
            <NoData v-else-if="!loading && !error" />
        </template>

        <template v-else-if="view === 'deployments'">
            <Deployments :deployments="deployments" />
        </template>

        <template v-else-if="view === 'traces'">
            <Traces v-if="traces" :view="traces" :loading="loading" class="mt-5" />
            <NoData v-else-if="!loading && !error" />
        </template>

        <template v-else-if="view === 'costs'">
            <v-alert v-if="!loading && !error && !costs" color="info" outlined text>
                Coroot currently supports cost monitoring for services running on AWS, GCP, and Azure. The agent on each node requires access to the
                cloud metadata service to obtain instance metadata, such as region, availability zone, and instance type.
            </v-alert>
            <NodesCosts v-if="costs && costs.nodes" :nodes="costs.nodes" class="mt-5" />
            <ApplicationsCosts v-if="costs && costs.applications" :applications="costs.applications" class="mt-5" />
        </template>
    </div>
</template>

<script>
import ServiceMap from '../components/ServiceMap';
import Table from '../components/Table';
import NoData from '../components/NoData';
import NodesCosts from '../components/NodesCosts';
import ApplicationsCosts from '../components/ApplicationsCosts';
import Deployments from './Deployments.vue';
import Health from './Health.vue';
import Traces from './Traces.vue';
import AgentInstallation from './AgentInstallation.vue';

export default {
    components: { Deployments, NoData, ServiceMap, Table, NodesCosts, ApplicationsCosts, Health, Traces, AgentInstallation },
    props: {
        view: String,
    },

    data() {
        return {
            health: null,
            map: null,
            nodes: null,
            deployments: null,
            traces: null,
            costs: null,
            categories: null,
            loading: false,
            error: '',
        };
    },

    computed: {
        views() {
            return {
                health: 'Health',
                map: 'Service Map',
                traces: 'Traces',
                nodes: 'Nodes',
                deployments: 'Deployments',
                costs: 'Costs',
            };
        },
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        view() {
            this.get();
        },
        $route(curr, prev) {
            if (curr.query.from === prev.query.from && curr.query.to === prev.query.to && curr.query.query !== prev.query.query) {
                this.get();
            }
        },
    },

    methods: {
        get() {
            const view = this.view || 'health';
            const query = this.$route.query.query || '';
            this.loading = true;
            this.error = '';
            this.$api.getOverview(view, query, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.categories = data.categories;
                this.health = data.health;
                this.map = data.map;
                this.nodes = data.nodes;
                this.deployments = data.deployments;
                this.traces = data.traces;
                this.costs = data.costs;
                if (!this.views[view]) {
                    this.$router.replace({ params: { view: undefined } }).catch((err) => err);
                }
            });
        },
    },
};
</script>

<style scoped>
.disabled {
    pointer-events: none;
}
</style>
