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

        <v-alert v-if="custom_pricing" color="info" outlined text>
            The nodes are either not in a supported cloud, or the agent cannot access cloud metadata. <br />
            In this case, custom pricing is used, and you can adjust the prices per vCPU and per GB of memory.
            <CustomCloudPricing />
        </v-alert>

        <NodesCosts v-if="nodes.length" :nodes="nodes" />
        <ApplicationsCosts v-if="applications.length" :applications="applications" />
    </div>
</template>

<script>
import NodesCosts from '@/components/NodesCosts.vue';
import ApplicationsCosts from '@/components/ApplicationsCosts.vue';
import CustomCloudPricing from '@/components/CustomCloudPricing.vue';

export default {
    components: { ApplicationsCosts, NodesCosts, CustomCloudPricing },

    data() {
        return {
            nodes: [],
            applications: [],
            loading: false,
            error: '',
            custom_pricing: false,
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
                this.custom_pricing = data.costs.custom_pricing;
                this.nodes = data.costs.nodes || [];
                this.applications = data.costs.applications || [];
            });
        },
    },
};
</script>
