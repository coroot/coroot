<template>
    <div>
        <h1 class="text-h5 my-5">
            Overview
            <v-progress-circular v-if="loading" indeterminate color="green" />
        </h1>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

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

        <template v-if="!view">
            <Health v-if="health" :applications="health" />
            <NoData v-else-if="!loading" />
        </template>

        <template v-else-if="view === 'map'">
            <ServiceMap v-if="map" :applications="map" />
            <NoData v-else-if="!loading" />
        </template>

        <template v-else-if="view === 'nodes'">
            <Table v-if="nodes && nodes.rows" :header="nodes.header" :rows="nodes.rows" />
            <NoData v-else-if="!loading" />
        </template>

        <template v-else-if="view === 'deployments'">
            <Deployments :deployments="deployments" />
        </template>

        <template v-else-if="view === 'costs'">
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

export default {
    components: { Deployments, NoData, ServiceMap, Table, NodesCosts, ApplicationsCosts, Health },
    props: {
        view: String,
    },

    data() {
        return {
            health: null,
            map: null,
            nodes: null,
            deployments: null,
            costs: null,
            loading: false,
            error: '',
        };
    },

    computed: {
        views() {
            return {
                health: 'Health',
                map: 'Service Map',
                nodes: 'Nodes',
                deployments: 'Deployments',
                costs: this.costs ? 'Costs' : '',
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
    },

    methods: {
        get() {
            const view = this.view || 'health';
            this.loading = true;
            this.error = '';
            this.$api.getOverview(view, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.health = data.health;
                this.map = data.map;
                this.nodes = data.nodes;
                this.costs = data.costs;
                this.deployments = data.deployments;
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
