<template>
    <div>
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

        <template v-if="view === 'kubernetes'">
            <Kubernetes />
        </template>

        <template v-if="view === 'traces'">
            <Traces />
        </template>

        <template v-if="view === 'logs'">
            <Logs />
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

        <template v-if="view === 'dashboards'">
            <Dashboard v-if="id" :id="id" />
            <Dashboards v-else />
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
import Logs from '@/views/Logs.vue';
import Nodes from '@/views/Nodes.vue';
import Node from '@/views/Node.vue';
import Costs from '@/views/Costs.vue';
import Anomalies from '@/views/Anomalies.vue';
import RCA from '@/views/RCA.vue';
import Risks from '@/views/Risks.vue';
import Dashboards from '@/views/dashboards/Dashboards.vue';
import Dashboard from '@/views/dashboards/Dashboard.vue';
import Kubernetes from '@/views/Kubernetes.vue';

export default {
    components: {
        Kubernetes,
        Applications,
        Application,
        Incidents,
        Incident,
        ServiceMap,
        Traces,
        Logs,
        Nodes,
        Node,
        Costs,
        Anomalies,
        RCA,
        Risks,
        Dashboards,
        Dashboard,
    },
    props: {
        view: String,
        id: String,
        report: String,
    },
};
</script>

<style scoped></style>
