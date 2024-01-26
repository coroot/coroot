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
                <template v-if="status.node_agent.status === 'unknown'"> unknown </template>
                <template v-else>
                    <template v-if="status.node_agent.nodes"> {{ $pluralize('node', status.node_agent.nodes, true) }} found </template>
                    <template v-else>
                        <template v-if="loading">checking...</template>
                        <template v-else>no agent installed</template>
                        <v-btn small icon @click="get" :loading="loading"><v-icon size="20">mdi-refresh</v-icon></v-btn>
                    </template>
                </template>
                (<a href="https://coroot.com/docs/metric-exporters/node-agent/installation" target="_blank">docs</a>)
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
                    <v-btn small icon @click="get" :loading="loading"><v-icon size="20">mdi-refresh</v-icon></v-btn>
                </template>
                (<a href="https://coroot.com/docs/metric-exporters/kube-state-metrics" target="_blank">docs</a>)
            </div>

            <div v-for="ex in exporters" :key="ex.type" class="mt-2" :class="{ muted: ex.muted }">
                <Led :status="ex.status" />
                <span class="font-weight-medium">{{ ex.type }}</span>
                <v-btn x-small color="primary" depressed class="ml-1" @click="save(!ex.muted, ex.type)" :loading="saving">
                    <v-tooltip bottom>
                        <template #activator="{ on }">
                            <v-icon v-on="on" small>mdi-volume-{{ ex.muted ? 'high' : 'off' }}</v-icon>
                        </template>
                        {{ ex.muted ? 'unmute' : 'mute' }}
                    </v-tooltip>
                </v-btn>
                (<a :href="`https://coroot.com/docs/metric-exporters/${ex.instruction.exporter}`" target="_blank">docs</a>)
                <div class="ml-5">
                    <span v-if="ex.instruction.description" v-html="ex.instruction.description" />
                    <template v-else>
                        <span class="text-capitalize">{{ ex.type }}</span> metrics have been received from {{ ex.instrumentedApps }} of
                        {{ $pluralize('application', ex.totalApps, true) }}.
                        <template v-if="ex.instrumentedApps < ex.totalApps">
                            Get <span class="font-weight-medium">{{ ex.instruction.exporter }} </span>
                            <a :href="`https://coroot.com/docs/metric-exporters/${ex.instruction.exporter}/installation`" target="_blank"
                                >installed</a
                            >
                            for every following application:
                        </template>
                    </template>
                </div>
                <div v-for="(ok, id) in ex.applications" :key="id" class="ml-5">
                    <Led :status="ok ? 'ok' : 'warning'" />
                    <router-link :to="{ name: 'application', params: { id } }">{{ $utils.appId(id).name }}</router-link>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
import Led from '../components/Led.vue';

export default {
    props: {
        projectId: String,
    },

    components: { Led },

    data() {
        return {
            status: null,
            error: null,
            loading: false,
            saving: false,
        };
    },

    computed: {
        exporters() {
            if (!this.status || !this.status.application_exporters) {
                return [];
            }
            const instructions = {
                postgres: { exporter: 'pg-agent' },
                redis: { exporter: 'redis-exporter' },
                mongodb: { exporter: 'mongodb-exporter' },
                'aws-rds': {
                    exporter: 'aws-agent',
                    description:
                        'It appears that AWS RDS is being used in this project. Please ensure that <a href="https://coroot.com/docs/metric-exporters/aws-agent/installation">aws-agent</a> is installed.',
                },
                'aws-elasticache': {
                    exporter: 'aws-agent',
                    description:
                        'It appears that AWS ElastiCache is being used in this project. Please ensure that <a href="https://coroot.com/docs/metric-exporters/aws-agent/installation">aws-agent</a> is installed.',
                },
            };
            const res = [];
            for (const type in this.status.application_exporters) {
                const ex = this.status.application_exporters[type];
                const totalApps = Object.keys(ex.applications).length;
                const instrumentedApps = Object.values(ex.applications).filter((ok) => ok).length;
                const instruction = instructions[type] || {};
                res.push({
                    ...ex,
                    type,
                    instruction,
                    totalApps,
                    instrumentedApps,
                });
            }
            return res;
        },
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
        save(mute, type) {
            this.saving = true;
            this.error = '';
            this.$api.setStatus(mute ? { mute: type } : { unmute: type }, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                }
                this.$events.emit('project-saved');
                this.get();
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
