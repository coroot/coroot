<template>
    <div style="max-width: 800px">
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>
        <div v-if="status">
            <div class="text-truncate">
                <Led :status="status.prometheus.status" />
                <span class="font-weight-medium">prometheus</span>:
                <span v-if="status.prometheus.error">
                    {{ status.prometheus.error }}
                </span>
                <span v-else>
                    {{ status.prometheus.message }}
                </span>
                <router-link v-if="status.prometheus.action === 'configure'" :to="{ params: { tab: 'prometheus' } }">configure</router-link>
            </div>

            <div class="d-flex align-center mt-2">
                <Led :status="status.node_agent.status" />
                <span class="font-weight-medium">coroot-node-agent</span>:
                <span class="ml-1 mr-2">
                    <template v-if="status.node_agent.status === 'unknown'"> unknown </template>
                    <template v-else>
                        <template v-if="status.node_agent.nodes"> {{ $pluralize('node', status.node_agent.nodes, true) }} found </template>
                        <template v-else>
                            <template v-if="loading">checking...</template>
                            <template v-else>no agent installed</template>
                        </template>
                    </template>
                </span>
                <AgentInstallation color="primary" small>Install</AgentInstallation>
            </div>

            <div v-if="status.kube_state_metrics" class="d-flex align-center mt-2">
                <Led :status="status.kube_state_metrics.status" />
                <span class="font-weight-medium">kube-state-metrics</span>:
                <template v-if="status.kube_state_metrics.status === 'ok'">
                    {{ $pluralize('application', status.kube_state_metrics.applications, true) }} found
                </template>
                <template v-else>
                    <template v-if="loading">checking...</template>
                    <template v-else>no kube-state-metrics installed</template>
                </template>
            </div>
        </div>
    </div>
</template>

<script>
import Led from '../components/Led.vue';
import AgentInstallation from './AgentInstallation.vue';

export default {
    props: {
        projectId: String,
    },

    components: { Led, AgentInstallation },

    data() {
        return {
            status: null,
            error: null,
            loading: false,
        };
    },

    mounted() {
        this.get();
    },

    watch: {
        projectId() {
            this.status = null;
            this.get();
        },
    },

    methods: {
        get() {
            if (!this.projectId) {
                return;
            }
            this.loading = true;
            this.$api.getStatus((data, error) => {
                setTimeout(() => {
                    this.loading = false;
                }, 500);
                if (error) {
                    this.error = error;
                    this.status = null;
                    return;
                }
                this.status = data;
                if (this.status.error) {
                    this.error = this.status.error;
                    this.status = null;
                }
            });
        },
    },
};
</script>

<style scoped>
.muted {
    color: grey;
}
</style>
