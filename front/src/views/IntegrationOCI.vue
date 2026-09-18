<template>
    <div>
        <p style="max-width: 800px">
            This integration enables Coroot to discover MySQL HeatWave and PostgreSQL DB systems and OCI Cache clusters and collect their telemetry
            data: instance status and OS metrics from OCI Monitoring. The cluster-agent authenticates through OKE Workload Identity or the instance
            principal of the nodes, so nothing is configured here: declare the integration and the database credentials in the Coroot custom resource
            or in the cluster-agent configuration file, see the
            <a href="https://docs.coroot.com/configuration/oci" target="_blank">OCI integration</a> page and the
            <a href="https://docs.coroot.com/guides/oci-oke-mysql" target="_blank">Monitoring MySQL HeatWave and OCI Cache from OKE</a> guide.
        </p>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-3" style="max-width: 800px">
            {{ error }}
        </v-alert>
        <CloudDiscovery provider="OCI" :configured="configured" :detected="detected" :errors="errors" :instances="instances" :error="error" />
    </div>
</template>

<script>
import CloudDiscovery from '../components/CloudDiscovery.vue';

export default {
    components: { CloudDiscovery },

    data() {
        return {
            error: '',
            configured: false,
            detected: false,
            errors: [],
            instances: [],
        };
    },

    mounted() {
        this.get();
    },

    methods: {
        get() {
            this.error = '';
            this.$api.getIntegrations('oci', (data, error) => {
                if (error) {
                    this.error = error;
                    return;
                }
                this.configured = !!data.view.configured;
                this.detected = !!data.view.detected;
                this.errors = data.view.errors || [];
                this.instances = data.view.instances || [];
            });
        },
    },
};
</script>
