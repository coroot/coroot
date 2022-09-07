<template>
    <div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{error}}
        </v-alert>
        <div v-if="status">
            <div class="text-truncate">
                <Led :status="status.prometheus.status" class="mr-1" />
                <span class="font-weight-medium">prometheus</span>:
                <span v-if="status.prometheus.error">
                    {{status.prometheus.error}}
                </span>
                <span v-else-if="status.prometheus.status !== 'ok'">
                    cache is {{$moment.duration(status.prometheus.lag, 'ms').format('h [hour] m [minute]', {trim: 'all'})}} behind
                </span>
                <span v-else>ok</span>
            </div>

            <div class="d-flex align-center">
                <Led :status="status.node_agent.status" class="mr-1" />
                <span class="font-weight-medium">coroot-node-agent</span>:
                <template v-if="status.node_agent.status === 'unknown'">
                    unknown
                </template>
                <template v-else>
                    <template v-if="status.node_agent.nodes">
                        {{$pluralize('node', status.node_agent.nodes, true)}} found
                    </template>
                    <template v-else>
                        <template v-if="loading">checking...</template>
                        <template v-else>no agent installed</template>
                        <v-btn small icon @click="get" :loading="loading"><v-icon size="20">mdi-refresh</v-icon></v-btn>
                    </template>
                </template>
                (<a href="" target="_blank">docs</a>)
            </div>

            <div v-if="status.kube_state_metrics" class="d-flex align-center">
                <Led :status="status.kube_state_metrics.status" class="mr-1" />
                kube-state-metrics:
                <template v-if="status.kube_state_metrics.status === 'ok'">
                    {{$pluralize('application', status.kube_state_metrics.applications, true)}} found
                </template>
                <template v-else>
                    <template v-if="loading">checking...</template>
                    <template v-else>no kube-state-metrics installed</template>
                    <v-btn small icon @click="get" :loading="loading"><v-icon size="20">mdi-refresh</v-icon></v-btn>
                </template>
                (<a href="">docs</a>)
            </div>
        </div>
    </div>
</template>

<script>
import Led from "@/components/Led";

export default {
    props: {
        projectId: String,
    },

    components: {Led},

    data() {
        return {
            status: null,
            error: null,
            loading: false,
        }
    },

    mounted() {
        this.get();
    },

    watch: {
        projectId() {
            this.status = null;
            this.get();
        }
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
            })
        },
    },
}
</script>

<style scoped>

</style>
