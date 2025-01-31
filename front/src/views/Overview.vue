<template>
    <div>
        <div class="my-4">
            <v-tabs :value="view" height="40" show-arrows slider-size="2">
                <template v-for="(name, view) in views">
                    <v-tab
                        v-if="name && view"
                        :to="{
                            params: { view, id: undefined, report: undefined },
                            query: view === 'incidents' ? { ...$utils.contextQuery(), incident: undefined } : $utils.contextQuery(),
                        }"
                        :tab-value="view"
                    >
                        {{ name }}
                    </v-tab>
                </template>
            </v-tabs>
        </div>

        <template v-if="view === 'applications'">
            <Application v-if="id" :id="id" :report="report" />
            <Applications v-else />
        </template>

        <template v-if="view === 'incidents'">
            <Incident v-if="$route.query.incident" />
            <Incidents v-else />
        </template>

        <template v-if="view === 'map'">
            <ServiceMap />
        </template>

        <template v-if="view === 'nodes'">
            <Node v-if="id" :name="id" />
            <Nodes v-else />
        </template>

        <template v-if="view === 'deployments'">
            <Deployments />
        </template>

        <template v-if="view === 'traces'">
            <Traces />
        </template>

        <template v-if="view === 'costs'">
            <Costs />
        </template>

        <template v-if="view === 'anomalies'">
            <RCA v-if="id" :appId="id" />
            <Anomalies v-else />
        </template>

        <template v-if="view === 'risks'">
            <Risks />
        </template>
    </div>
</template>

<script>
import Applications from '@/views/Applications.vue';
import Application from '@/views/Application.vue';
import Incidents from '@/views/Incidents.vue';
import Incident from '@/views/Incident.vue';
import ServiceMap from '@/views/ServiceMap.vue';
import Traces from '@/views/Traces.vue';
import Nodes from '@/views/Nodes.vue';
import Node from '@/views/Node.vue';
import Deployments from '@/views/Deployments.vue';
import Costs from '@/views/Costs.vue';
import Anomalies from '@/views/Anomalies.vue';
import RCA from '@/views/RCA.vue';
import Risks from '@/views/Risks.vue';

export default {
    components: {
        Applications,
        Application,
        Incidents,
        Incident,
        ServiceMap,
        Traces,
        Nodes,
        Node,
        Deployments,
        Costs,
        Anomalies,
        RCA,
        Risks,
    },
    props: {
        view: String,
        id: String,
        report: String,
    },

    computed: {
        views() {
            return {
                '': this.$route.query, // a bit of a hack to enable reactivity for tabs
                applications: 'Applications',
                incidents: 'Incidents',
                map: 'Service Map',
                traces: 'Traces',
                nodes: 'Nodes',
                deployments: 'Deployments',
                costs: 'Costs',
                anomalies: this.$coroot.edition === 'Enterprise' ? 'Anomalies' : '',
                risks: 'Risks',
            };
        },
    },

    watch: {
        view: {
            handler(v) {
                if (!this.views[v]) {
                    this.$router.replace({ params: { view: 'applications' } }).catch((err) => err);
                }
            },
            immediate: true,
        },
    },
};
</script>

<style scoped></style>
