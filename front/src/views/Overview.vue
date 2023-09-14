<template>
<div>
    <h1 class="text-h5 my-5">
        Overview
        <v-progress-circular v-if="loading" indeterminate color="green" />
    </h1>
    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
        {{error}}
    </v-alert>

    <v-tabs height="40" show-arrows slider-size="2" class="mb-3">
        <v-tab v-for="v in views" :key="v" :to="{params: {view: v === 'applications' ? undefined : v }, query: $utils.contextQuery()}" exact-path>
            {{ v }}
        </v-tab>
    </v-tabs>

    <template v-if="!view">
        <ServiceMap v-if="applications" :applications="applications" />
        <NoData v-else-if="!loading" />
    </template>

    <template v-else-if="view === 'nodes'">
        <Table v-if="nodes && nodes.rows" :header="nodes.header" :rows="nodes.rows" />
        <NoData v-else-if="!loading" />
    </template>

    <template v-else-if="view === 'costs'">
        <NodesCosts v-if="costs && costs.nodes" :nodes="costs.nodes" class="mt-5" />
        <ApplicationsCosts v-if="costs && costs.applications" :applications="costs.applications" class="mt-5" />
    </template>
</div>
</template>

<script>
import ServiceMap from "../components/ServiceMap";
import Table from "../components/Table";
import NoData from "../components/NoData";
import NodesCosts from "../components/NodesCosts";
import ApplicationsCosts from "../components/ApplicationsCosts";

export default {
    components: {NoData, ServiceMap, Table, NodesCosts, ApplicationsCosts},
    props: {
        view: String,
    },

    data() {
        return {
            views: ['applications', 'nodes'],
            applications: null,
            nodes: null,
            costs: null,
            loading: false,
            error: '',
        }
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
            const view = this.view || 'applications';
            this.loading = true;
            this.$api.getOverview(view, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.views = data.views || this.views;
                this.applications = data.applications;
                this.nodes = data.nodes;
                this.costs = data.costs;
                if (!this.views.find(v => v === view)) {
                    this.$router.replace({params: {view: undefined}}).catch(err => err);
                }
            });
        }
    },
};
</script>

<style scoped>
</style>