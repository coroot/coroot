<template>
    <div>
        <v-progress-linear indeterminate v-if="loading" color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <v-alert v-if="!loading && !error && !nodes.length" color="info" outlined text>
            Coroot currently supports cost monitoring for services running on AWS, GCP, and Azure. The agent on each node requires access to the cloud
            metadata service to obtain instance metadata, such as region, availability zone, and instance type.
        </v-alert>

        <NodesCosts v-if="nodes.length" :nodes="nodes" />
        <ApplicationsCosts v-if="applications.length" :applications="applications" />
    </div>
</template>

<script>
import NodesCosts from '@/components/NodesCosts.vue';
import ApplicationsCosts from '@/components/ApplicationsCosts.vue';

export default {
    components: { ApplicationsCosts, NodesCosts },

    data() {
        return {
            nodes: [],
            applications: [],
            loading: false,
            error: '',
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getOverview('costs', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.nodes = data.costs.nodes || [];
                this.applications = data.costs.applications || [];
            });
        },
    },
};
</script>
