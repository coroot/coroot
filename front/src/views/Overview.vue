<template>
    <div>
        <div class="my-4">
            <v-tabs v-model="tab" height="40" show-arrows slider-size="2">
                <template v-for="(name, id) in views">
                    <v-tab
                        :to="{
                            params: { view: id, app: undefined },
                            query: id === 'incidents' ? undefined : $utils.contextQuery(),
                        }"
                        :class="{ disabled: loading }"
                        :tab-value="id"
                    >
                        {{ name }}
                    </v-tab>
                </template>
            </v-tabs>
            <v-progress-linear indeterminate v-if="loading" color="green" class="mt-2" />
        </div>

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <v-tabs-items v-model="tab">
            <v-tab-item value="health" eager transition="none">
                <Health v-if="health" :applications="health" />
                <NoData v-else-if="!loading && !error" />
            </v-tab-item>

            <v-tab-item value="incidents" eager transition="none">
                <template v-if="$route.query.incident">
                    <Incident />
                </template>
                <template v-else>
                    <Incidents v-if="incidents" :incidents="incidents" />
                    <NoData v-else-if="!loading && !error" />
                </template>
            </v-tab-item>

            <v-tab-item value="map" eager transition="none">
                <ServiceMap v-if="map" :applications="map" :categories="categories" />
                <NoData v-else-if="!loading && !error" />
            </v-tab-item>

            <v-tab-item value="nodes" eager transition="none">
                <template v-if="nodes && nodes.rows">
                    <Table v-if="nodes && nodes.rows" :header="nodes.header" :rows="nodes.rows" />
                    <div class="mt-4">
                        <AgentInstallation color="primary">Add nodes</AgentInstallation>
                    </div>
                </template>
                <NoData v-else-if="!loading && !error" />
            </v-tab-item>

            <v-tab-item value="deployments" eager transition="none">
                <Deployments :deployments="deployments" />
            </v-tab-item>

            <v-tab-item value="traces" eager transition="none">
                <Traces v-if="traces" :view="traces" :loading="loading" />
                <NoData v-else-if="!loading && !error" />
            </v-tab-item>

            <v-tab-item value="costs" eager transition="none">
                <v-alert v-if="!loading && !error && !costs" color="info" outlined text>
                    Coroot currently supports cost monitoring for services running on AWS, GCP, and Azure. The agent on each node requires access to
                    the cloud metadata service to obtain instance metadata, such as region, availability zone, and instance type.
                </v-alert>
                <NodesCosts v-if="costs && costs.nodes" :nodes="costs.nodes" />
                <ApplicationsCosts v-if="costs && costs.applications" :applications="costs.applications" />
            </v-tab-item>

            <v-tab-item value="anomalies" eager transition="none">
                <template v-if="app">
                    <RCA :appId="app" />
                </template>
                <template v-else>
                    <Anomalies />
                </template>
            </v-tab-item>
        </v-tabs-items>
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
import Incidents from './Incidents.vue';
import Incident from './Incident.vue';
import RCA from '@/views/RCA.vue';
import Anomalies from '@/views/Anomalies.vue';

export default {
    components: {
        Anomalies,
        RCA,
        Incident,
        Incidents,
        Deployments,
        NoData,
        ServiceMap,
        Table,
        NodesCosts,
        ApplicationsCosts,
        Health,
        Traces,
        AgentInstallation,
    },
    props: {
        view: String,
        app: String,
    },

    data() {
        return {
            tab: this.view,
            health: null,
            incidents: null,
            map: null,
            nodes: null,
            deployments: null,
            traces: null,
            costs: null,
            categories: null,
            loading: false,
            error: '',
            query: '',
        };
    },

    computed: {
        views() {
            const res = {
                health: 'Health',
                incidents: 'Incidents',
                map: 'Service Map',
                traces: 'Traces',
                nodes: 'Nodes',
                deployments: 'Deployments',
                costs: 'Costs',
            };
            if (this.$coroot.edition === 'Enterprise') {
                res.anomalies = 'Anomalies';
            }
            return res;
        },
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        view() {
            if (!this.view) {
                this.tab = 'health';
            }
            this.get();
        },
        $route(curr, prev) {
            if (
                curr.query.from === prev.query.from &&
                curr.query.to === prev.query.to &&
                (curr.query.query !== prev.query.query || curr.query.incident !== prev.query.incident)
            ) {
                this.get();
            }
        },
    },

    methods: {
        get() {
            if (this.view === 'incidents' && this.$route.query.incident) {
                return;
            }
            if (this.view === 'anomalies') {
                return;
            }
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
                this.incidents = data.incidents || [];
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
