<template>
    <v-dialog v-model="dialog" width="80%">
        <v-card class="pa-5">
            <div class="d-flex align-center mb-4">
                <div v-if="loading">Loading...</div>
                <div v-else class="d-flex align-center">
                    <span class="font-weight-medium">Alert</span>
                    <span class="ml-2" style="font-family: monospace">{{ alertId }}</span>
                    <v-chip v-if="alert" small label class="ml-2" :color="statusColor" :text-color="statusTextColor">
                        {{ statusLabel }}
                    </v-chip>
                </div>
                <v-spacer />
                <v-btn icon @click="close"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <div v-if="loading" class="py-4">
                <v-progress-linear indeterminate />
            </div>

            <template v-else-if="alert">
                <div class="detail-grid">
                    <div class="label">Rule</div>
                    <div>
                        <router-link
                            v-if="alert.rule_id"
                            :to="{
                                name: 'overview',
                                params: { view: 'alerts', id: 'rules' },
                                query: { ...$utils.contextQuery(), rule: alert.rule_id },
                            }"
                            @click.native="close"
                        >
                            {{ alert.rule_name || alert.rule_id }}
                        </router-link>
                        <span v-else class="grey--text">-</span>
                    </div>

                    <template v-if="$utils.appId(alert.application_id).name">
                        <div class="label">Application</div>
                        <div>
                            <router-link
                                :to="{
                                    name: 'overview',
                                    params: { view: 'applications', id: alert.application_id, report: alert.report || undefined },
                                    query: {
                                        ...alertContextQuery,
                                        ...(alert.report === 'Logs' ? { query: JSON.stringify({ view: 'patterns', source: 'agent' }) } : {}),
                                    },
                                }"
                                @click.native="close"
                            >
                                {{ $utils.appId(alert.application_id).name }}
                            </router-link>
                        </div>
                    </template>

                    <div class="label">Severity</div>
                    <div>
                        <v-chip small label outlined :color="severityColor">
                            {{ alert.severity === 'critical' ? 'Critical' : 'Warning' }}
                        </v-chip>
                    </div>

                    <div class="label">Opened at</div>
                    <div>
                        {{ $format.date(alert.opened_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}
                        ({{ $format.timeSinceNow(alert.opened_at) }} ago)
                    </div>

                    <div class="label">Duration</div>
                    <div>
                        <template v-if="alert.resolved_at">
                            {{ $format.durationPretty(alert.duration) }}
                        </template>
                        <template v-else> {{ $format.timeSinceNow(alert.opened_at) }} (ongoing) </template>
                    </div>

                    <template v-if="alert.suppressed">
                        <div class="label">Suppressed by</div>
                        <div>{{ alert.resolved_by || 'unknown' }}</div>
                    </template>
                    <template v-else-if="alert.manually_resolved_at">
                        <div class="label">Resolved at</div>
                        <div>{{ $format.date(alert.manually_resolved_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}</div>
                        <template v-if="alert.resolved_by">
                            <div class="label">Resolved by</div>
                            <div>{{ alert.resolved_by }}</div>
                        </template>
                    </template>
                    <template v-else-if="alert.resolved_at">
                        <div class="label">Resolved at</div>
                        <div>{{ $format.date(alert.resolved_at, '{MMM} {DD}, {HH}:{mm}:{ss}') }}</div>
                    </template>

                    <div class="label">Summary</div>
                    <div>{{ alert.summary }}</div>

                    <template v-if="alert.details && alert.details.length">
                        <template v-for="(d, i) in alert.details">
                            <template v-if="d.name === 'PromQL'">
                                <div :key="'dl-' + i" class="label">Query</div>
                                <div :key="'dv-' + i" class="description log-sample">
                                    <span v-text="d.value" /><v-btn
                                        icon
                                        x-small
                                        class="copy-btn"
                                        @click="copyText(d.value)"
                                        :title="copiedText === d.value ? 'Copied' : 'Copy'"
                                    >
                                        <v-icon small>{{ copiedText === d.value ? 'mdi-check' : 'mdi-content-copy' }}</v-icon>
                                    </v-btn>
                                </div>
                            </template>
                            <template v-else-if="d.name === 'PromQLChart'">
                                <div :key="'dl-' + i" class="label">Chart</div>
                                <div :key="'dv-' + i">
                                    <Panel :config="promqlPanelConfig(d.value)" style="height: 200px" />
                                </div>
                            </template>
                            <template v-else>
                                <div :key="'dl-' + i" class="label">{{ d.name }}</div>
                                <div :key="'dv-' + i" :class="d.code ? 'description log-sample' : 'description'">
                                    <span v-text="d.value" /><v-btn
                                        v-if="d.code"
                                        icon
                                        x-small
                                        class="copy-btn"
                                        @click="copyText(d.value)"
                                        :title="copiedText === d.value ? 'Copied' : 'Copy'"
                                    >
                                        <v-icon small>{{ copiedText === d.value ? 'mdi-check' : 'mdi-content-copy' }}</v-icon>
                                    </v-btn>
                                </div>
                            </template>
                        </template>
                    </template>

                    <div class="label">Notifications</div>
                    <div>
                        <div v-if="alert.notifications && alert.notifications.length" class="d-flex flex-wrap" style="gap: 4px">
                            <span v-for="(n, i) in alert.notifications" :key="i" class="notification-badge">
                                {{ n.type }}{{ n.channel ? ': #' + n.channel : '' }}
                            </span>
                        </div>
                        <span v-else class="grey--text">-</span>
                    </div>
                </div>

                <Dashboard v-if="alert.widgets && alert.widgets.length" :name="alert.report || ''" :widgets="alert.widgets" class="mt-2" />

                <v-btn v-if="alert.log_pattern_hash" small color="primary" :to="logMessagesLink" @click.native="close" class="mt-2">
                    Show messages
                </v-btn>

                <v-alert v-if="error" color="error" icon="mdi-alert-octagon-outline" outlined text class="mt-4">
                    {{ error }}
                </v-alert>

                <div class="d-flex align-center mt-4" style="gap: 8px">
                    <v-btn
                        v-if="isFiring"
                        small
                        outlined
                        :loading="resolving"
                        @click="resolve"
                        title="Acknowledge the alert; it will reopen if the condition recurs"
                    >
                        <v-icon small class="mr-1">mdi-check-circle-outline</v-icon>
                        Resolve
                    </v-btn>
                    <v-btn
                        v-if="isSuppressible"
                        small
                        outlined
                        :loading="suppressing"
                        @click="suppress"
                        title="Permanently silence the alert; it will not reopen until manually reopened"
                    >
                        <v-icon small class="mr-1">mdi-bell-off-outline</v-icon>
                        Suppress
                    </v-btn>
                    <v-btn v-if="isReopenable" small outlined :loading="reopening" @click="reopen" title="Reopen the alert so it can fire again">
                        <v-icon small class="mr-1">mdi-restore</v-icon>
                        Reopen
                    </v-btn>
                    <v-spacer />
                    <v-btn small outlined @click="close">Close</v-btn>
                </div>
            </template>

            <v-alert v-else-if="error" color="error" icon="mdi-alert-octagon-outline" outlined text>
                {{ error }}
            </v-alert>
        </v-card>
    </v-dialog>
</template>

<script>
import Panel from '@/views/dashboards/Panel.vue';
import Dashboard from '@/components/Dashboard.vue';

export default {
    components: { Panel, Dashboard },

    props: {
        alertId: {
            type: String,
            required: true,
        },
    },

    data() {
        return {
            dialog: true,
            loading: false,
            resolving: false,
            suppressing: false,
            reopening: false,
            error: '',
            alert: null,
            copiedText: null,
        };
    },

    computed: {
        statusColor() {
            if (!this.alert) return 'grey';
            if (this.alert.suppressed || this.alert.manually_resolved_at || this.alert.resolved_at) return 'grey lighten-1';
            if (this.alert.severity === 'critical') return 'red lighten-1';
            return 'orange lighten-1';
        },
        statusTextColor() {
            return 'white';
        },
        statusLabel() {
            if (!this.alert) return '';
            if (this.alert.suppressed) return 'Suppressed';
            if (this.alert.manually_resolved_at) return 'Resolved';
            if (this.alert.resolved_at) return 'Resolved';
            return 'Firing';
        },
        isFiring() {
            if (!this.alert) return false;
            return !this.alert.resolved_at && !this.alert.manually_resolved_at && !this.alert.suppressed;
        },
        isSuppressible() {
            if (!this.alert) return false;
            return !this.alert.suppressed;
        },
        isReopenable() {
            if (!this.alert) return false;
            return this.alert.manually_resolved_at || this.alert.suppressed;
        },
        severityColor() {
            if (!this.alert) return 'grey';
            return this.alert.severity === 'critical' ? 'red lighten-1' : 'orange lighten-1';
        },
        alertContextQuery() {
            if (!this.alert) return { alert: this.alertId };
            return { alert: this.alert.id };
        },
        logMessagesLink() {
            if (!this.alert || !this.alert.log_pattern_hash) return null;
            const query = JSON.stringify({
                view: 'messages',
                source: 'agent',
                filters: [{ name: 'pattern.hash', op: '=', value: this.alert.log_pattern_hash }],
            });
            return {
                name: 'overview',
                params: { view: 'applications', id: this.alert.application_id, report: 'Logs' },
                query: { ...this.alertContextQuery, query },
            };
        },
    },

    watch: {
        dialog(v) {
            if (!v) {
                this.$emit('close');
            }
        },
    },

    mounted() {
        this.load();
    },

    methods: {
        load() {
            this.loading = true;
            this.error = '';
            this.$api.getAlert(this.alertId, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.alert = data;
            });
        },
        resolve() {
            this.resolving = true;
            this.error = '';
            this.$api.resolveAlerts([this.alertId], (data, error) => {
                this.resolving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$emit('updated');
                this.close();
            });
        },
        suppress() {
            this.suppressing = true;
            this.error = '';
            this.$api.suppressAlerts([this.alertId], (data, error) => {
                this.suppressing = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$emit('updated');
                this.close();
            });
        },
        reopen() {
            this.reopening = true;
            this.error = '';
            this.$api.reopenAlerts([this.alertId], (data, error) => {
                this.reopening = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$emit('updated');
                this.close();
            });
        },
        promqlPanelConfig(query) {
            return {
                name: '',
                description: '',
                source: {
                    metrics: {
                        queries: [{ query: query, legend: '', color: '', datasource: '' }],
                    },
                },
                widget: { chart: {} },
            };
        },
        copyText(text) {
            navigator.clipboard.writeText(text).then(() => {
                this.copiedText = text;
                setTimeout(() => {
                    this.copiedText = null;
                }, 2000);
            });
        },
        close() {
            this.dialog = false;
        },
    },
};
</script>

<style scoped>
.detail-grid {
    display: grid;
    grid-template-columns: 120px 1fr;
    gap: 8px 16px;
    align-items: start;
    font-size: 16px;
}
.detail-grid .label {
    color: var(--text-color-dimmed);
}
.description {
    white-space: pre-wrap;
}
.description.log-sample {
    position: relative;
    font-family: monospace, monospace;
    background-color: var(--background-color-hi);
    filter: brightness(var(--brightness));
    border-radius: 3px;
    max-height: 50vh;
    padding: 8px;
    padding-right: 32px;
    overflow: auto;
    white-space: pre;
}
.description.log-sample .copy-btn {
    position: absolute;
    top: 4px;
    right: 4px;
    opacity: 0.5;
}
.description.log-sample .copy-btn:hover {
    opacity: 1;
}
.notification-badge {
    font-size: 12px;
    padding: 1px 6px;
    border-radius: 3px;
    background-color: rgba(128, 128, 128, 0.15);
    white-space: nowrap;
}
</style>
