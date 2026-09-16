<template>
    <div>
        <p style="max-width: 800px">
            This integration enables Coroot to discover Cloud SQL and Memorystore instances and collect their telemetry data: instance status, OS
            metrics from Cloud Monitoring, and database logs from Cloud Logging. The cluster-agent authenticates through GKE Workload Identity, so
            nothing is configured here: declare the integration and the database credentials in the Coroot custom resource or in the cluster-agent
            configuration file, see the <a href="https://docs.coroot.com/configuration/gcp" target="_blank">GCP integration</a> page and the
            <a href="https://docs.coroot.com/guides/gcp-gke-cloudsql" target="_blank">Monitoring Cloud SQL and Memorystore from GKE</a> guide.
        </p>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-3" style="max-width: 800px">
            {{ error }}
        </v-alert>
        <CloudDiscovery
            provider="Google Cloud"
            :configured="configured"
            :detected="detected"
            :errors="errors"
            :instances="instances"
            :error="error"
        />
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
            this.$api.getIntegrations('gcp', (data, error) => {
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
