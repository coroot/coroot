<template>
    <v-dialog v-model="dialog" max-width="900" scrollable>
        <v-card style="max-height: 90vh; display: flex; flex-direction: column">
            <div class="d-flex align-center font-weight-medium pa-5 pb-0">
                <div v-if="loading">Loading...</div>
                <div v-else-if="confirmingDelete">Delete Alerting Rule</div>
                <div v-else-if="ruleId">Edit Alerting Rule</div>
                <div v-else>Create Alerting Rule</div>
                <v-spacer />
                <v-btn icon @click="close"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <div v-if="loading" class="pa-5">
                <v-progress-linear indeterminate />
            </div>

            <v-form
                v-else
                ref="form"
                v-model="valid"
                :disabled="confirmingDelete || isReadonly"
                class="d-flex flex-column"
                style="overflow: hidden; flex: 1; min-height: 0"
            >
                <div class="form-content">
                    <v-alert v-if="isReadonly" color="info" icon="mdi-lock-outline" outlined text class="mb-4">
                        This rule is managed via config file and cannot be edited here.
                    </v-alert>
                    <div v-if="ruleId" class="d-flex align-center mb-4">
                        <span class="caption grey--text">ID:</span>
                        <code class="ml-1">{{ ruleId }}</code>
                    </div>
                    <div class="subtitle-1">Name</div>
                    <v-text-field v-model="name" :rules="[$validators.notEmpty]" outlined dense hide-details="auto" class="mb-4" />

                    <div class="subtitle-1">Data source</div>
                    <v-select v-model="sourceType" :items="sourceTypes" outlined dense hide-details="auto" class="mb-4" />

                    <template v-if="sourceType === 'check'">
                        <div class="subtitle-1">Inspection</div>
                        <div class="caption grey--text">The inspection that will trigger alerts when its condition is met.</div>
                        <v-select
                            v-model="checkId"
                            :items="checkOptions"
                            :rules="[$validators.notEmpty]"
                            outlined
                            dense
                            hide-details="auto"
                            class="mb-4"
                        />
                    </template>

                    <template v-if="sourceType === 'log_patterns'">
                        <div class="subtitle-1">Log severities</div>
                        <div class="caption grey--text">Which log severity levels to monitor for new patterns.</div>
                        <v-select
                            v-model="logPatternSeverities"
                            :items="logSeverityOptions"
                            multiple
                            outlined
                            dense
                            hide-details="auto"
                            class="mb-4"
                        />

                        <div class="mb-4">
                            <div class="subtitle-1">Min message count</div>
                            <div class="caption grey--text">Minimum number of messages for a pattern to trigger an alert.</div>
                            <v-text-field
                                v-model.number="logPatternMinCount"
                                type="number"
                                :rules="[validatePositiveInt]"
                                outlined
                                dense
                                hide-details="auto"
                            />
                        </div>

                        <div class="mb-4">
                            <div class="subtitle-1">Max alerts per app</div>
                            <div class="caption grey--text">Maximum number of concurrent alerts per application.</div>
                            <v-text-field
                                v-model.number="logPatternMaxAlertsPerApp"
                                type="number"
                                :rules="[validatePositiveInt]"
                                outlined
                                dense
                                hide-details="auto"
                            />
                        </div>

                        <v-checkbox
                            v-model="logPatternEvaluateWithAI"
                            :disabled="$coroot.edition !== 'Enterprise'"
                            color="primary"
                            hide-details
                            class="mt-0 pt-0 mb-1"
                        >
                            <template #label>
                                <span>Evaluate with AI</span>
                            </template>
                        </v-checkbox>
                        <div class="caption grey--text mb-4">
                            Every log pattern will be analyzed by AI to determine whether it's worth notifying the team, and important errors will
                            include a brief explanation.
                            <template v-if="$coroot.edition !== 'Enterprise'">
                                Available in <a href="https://coroot.com/editions" target="_blank">Coroot Enterprise</a>.
                            </template>
                        </div>
                    </template>

                    <template v-if="sourceType === 'promql'">
                        <div class="subtitle-1">PromQL expression</div>
                        <div class="caption grey--text">
                            The expression to evaluate. Each resulting time series with a non-zero value fires an alert.
                        </div>
                        <MetricSelector v-model="promqlExpression" :rules="[$validators.notEmpty]" :debounce="500" class="mb-4" />

                        <div class="subtitle-1">Preview</div>
                        <Panel :config="previewPanelConfig" style="height: 200px" class="mb-4" />

                        <div class="subtitle-1">Summary template</div>
                        <div class="caption grey--text">
                            Optional summary using Go template syntax. Available variables:
                            <code v-pre>{{.value}}</code
                            >, <code v-pre>{{.labels.instance}}</code
                            >, or <code v-pre>{{.instance}}</code> for direct label access.
                        </div>
                        <v-text-field
                            v-model="templateSummary"
                            outlined
                            dense
                            hide-details="auto"
                            :placeholder="'Instance {{.instance}} is down'"
                            class="mb-4"
                        />
                    </template>

                    <template v-if="sourceType !== 'promql'">
                        <div class="subtitle-1">Application selector</div>
                        <div class="caption grey--text">Which applications this rule applies to.</div>
                        <v-select v-model="selectorType" :items="selectorTypes" outlined dense hide-details="auto" class="mb-4" />

                        <template v-if="selectorType === 'category'">
                            <div class="subtitle-1">Categories</div>
                            <div class="caption grey--text">Select the application categories to include.</div>
                            <v-select
                                v-model="selectorCategories"
                                :items="categoryOptions"
                                multiple
                                outlined
                                dense
                                hide-details="auto"
                                class="mb-4"
                            />
                        </template>

                        <template v-if="selectorType === 'applications'">
                            <div class="subtitle-1">Application patterns</div>
                            <div class="caption grey--text">Comma-separated glob patterns to match application IDs (e.g., */myapp, namespace/*).</div>
                            <v-text-field v-model="selectorPatternsText" outlined dense hide-details="auto" class="mb-4" />
                        </template>
                    </template>

                    <div class="subtitle-1">Severity</div>
                    <div class="caption grey--text mb-2">Alert severity level when this rule fires.</div>
                    <div class="d-flex mb-4" style="gap: 8px">
                        <v-chip
                            v-for="opt in severityOptions"
                            :key="opt.value"
                            label
                            :color="severity === opt.value ? opt.color : 'grey lighten-2'"
                            :text-color="severity === opt.value ? 'white' : 'grey darken-1'"
                            @click="severity = opt.value"
                            style="cursor: pointer"
                        >
                            {{ opt.text }}
                        </v-chip>
                    </div>

                    <template v-if="sourceType === 'promql'">
                        <div class="subtitle-1">Notification category</div>
                        <div class="caption grey--text">Which category's notification settings to use for this rule.</div>
                        <v-select v-model="notificationCategory" :items="categoryOptions" outlined dense hide-details="auto" class="mb-4" />
                    </template>

                    <v-row dense class="mb-4">
                        <v-col cols="6" class="d-flex flex-column">
                            <div class="subtitle-1">For</div>
                            <div class="caption grey--text flex-grow-1">How long the condition must be true before firing (e.g., 5m, 1h).</div>
                            <v-text-field v-model="forDuration" :rules="[validateDuration]" placeholder="0" outlined dense hide-details="auto" />
                        </v-col>
                        <v-col cols="6" class="d-flex flex-column">
                            <div class="subtitle-1">Keep firing for</div>
                            <div class="caption grey--text flex-grow-1">How long to keep the alert active after condition clears (e.g., 5m, 1h).</div>
                            <v-text-field
                                v-model="keepFiringForDuration"
                                :rules="[validateDuration]"
                                placeholder="0"
                                outlined
                                dense
                                hide-details="auto"
                            />
                        </v-col>
                    </v-row>

                    <div class="subtitle-1">Description template</div>
                    <div class="caption grey--text">Optional detailed description of the alert and suggested actions.</div>
                    <v-textarea v-model="templateDescription" outlined dense hide-details="auto" rows="3" class="mb-4" />

                    <v-checkbox v-model="enabled" color="primary" hide-details class="mt-0 pt-0 mb-1">
                        <template #label>
                            <span>Enabled</span>
                        </template>
                    </v-checkbox>
                    <div class="caption red--text mb-4">Disabling the rule will resolve all its firing alerts.</div>

                    <v-alert v-if="error" color="error" icon="mdi-alert-octagon-outline" outlined text class="my-4">
                        {{ error }}
                    </v-alert>
                </div>

                <div class="d-flex align-center pa-5 pt-4" style="flex-shrink: 0">
                    <v-btn v-if="canDelete && !confirmingDelete" color="error" text @click="confirmingDelete = true">Delete</v-btn>
                    <v-spacer />
                    <template v-if="confirmingDelete">
                        <v-btn color="error" :loading="saving" @click="deleteRule">Delete</v-btn>
                        <v-btn outlined class="ml-2" @click="confirmingDelete = false">Cancel</v-btn>
                    </template>
                    <template v-else-if="isReadonly">
                        <v-btn outlined @click="close">Close</v-btn>
                    </template>
                    <template v-else>
                        <v-btn color="primary" :disabled="!valid" :loading="saving" @click="save">
                            {{ ruleId ? 'Save' : 'Create' }}
                        </v-btn>
                        <v-btn outlined class="ml-2" @click="close">Cancel</v-btn>
                    </template>
                </div>
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script>
import MetricSelector from '@/components/MetricSelector.vue';
import Panel from '@/views/dashboards/Panel.vue';

export default {
    components: { MetricSelector, Panel },

    props: {
        ruleId: {
            type: String,
            default: null,
        },
        checks: {
            type: Array,
            default: () => [],
        },
        categories: {
            type: Array,
            default: () => [],
        },
    },

    data() {
        return {
            dialog: true,
            loading: false,
            saving: false,
            error: '',
            valid: false,
            rule: null,
            confirmingDelete: false,
            name: '',
            sourceType: 'check',
            checkId: '',
            logPatternSeverities: ['warning', 'error', 'fatal'],
            logPatternMinCount: 10,
            logPatternMaxAlertsPerApp: 20,
            logPatternEvaluateWithAI: false,
            selectorType: 'all',
            selectorCategories: [],
            selectorPatternsText: '',
            severity: 'warning',
            forDuration: '',
            keepFiringForDuration: '',
            templateDescription: '',
            enabled: true,
            promqlExpression: '',
            templateSummary: '',
            notificationCategory: 'application',
            sourceTypes: [
                { value: 'check', text: 'Built-in inspection' },
                { value: 'log_patterns', text: 'Log patterns' },
                { value: 'promql', text: 'PromQL expression' },
            ],
            selectorTypes: [
                { value: 'all', text: 'All applications' },
                { value: 'category', text: 'By category' },
                { value: 'applications', text: 'By application patterns' },
            ],
            severityOptions: [
                { value: 'warning', text: 'Warning', color: 'orange lighten-1' },
                { value: 'critical', text: 'Critical', color: 'red lighten-1' },
            ],
            logSeverityOptions: [
                { value: 'warning', text: 'Warning' },
                { value: 'error', text: 'Error' },
                { value: 'fatal', text: 'Fatal' },
            ],
        };
    },

    computed: {
        checkOptions() {
            return this.checks.map((c) => ({ value: c.id, text: c.title })).sort((a, b) => a.text.localeCompare(b.text));
        },
        categoryOptions() {
            return this.categories.map((c) => ({ value: c.name, text: c.name })).sort((a, b) => a.text.localeCompare(b.text));
        },
        isReadonly() {
            return this.rule && this.rule.readonly;
        },
        canDelete() {
            return this.ruleId && this.rule && !this.rule.builtin && !this.rule.readonly;
        },
        previewPanelConfig() {
            return {
                name: '',
                description: '',
                source: {
                    metrics: {
                        queries: [{ query: this.promqlExpression, legend: '', color: '', datasource: '' }],
                    },
                },
                widget: { chart: {} },
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
        if (this.ruleId) {
            this.loadRule();
        }
    },

    methods: {
        parseDuration(str) {
            if (!str || str === '0' || str === '') {
                return 0;
            }
            const match = str.match(/^(\d+)(s|m|h|d)$/);
            if (!match) {
                const num = parseInt(str, 10);
                return isNaN(num) ? null : num * 1000;
            }
            const value = parseInt(match[1], 10);
            const unit = match[2];
            let seconds;
            switch (unit) {
                case 's':
                    seconds = value;
                    break;
                case 'm':
                    seconds = value * 60;
                    break;
                case 'h':
                    seconds = value * 3600;
                    break;
                case 'd':
                    seconds = value * 86400;
                    break;
                default:
                    return null;
            }
            return seconds * 1000;
        },
        validatePositiveInt(v) {
            if (v === '' || v === null || v === undefined) {
                return 'Required';
            }
            const n = Number(v);
            if (!Number.isInteger(n) || n < 1) {
                return 'Must be a positive integer';
            }
            return true;
        },
        validateDuration(v) {
            if (!v || v === '0' || v === '') {
                return true;
            }
            const match = v.match(/^(\d+)(s|m|h|d)?$/);
            if (!match) {
                return 'Invalid duration (use format: 5m, 1h, 30s)';
            }
            return true;
        },
        loadRule() {
            this.loading = true;
            this.error = '';
            this.$api.getAlertingRule(this.ruleId, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.rule = data;
                this.name = data.name || '';
                this.sourceType = (data.source && data.source.type) || 'check';
                this.checkId = (data.source && data.source.check && data.source.check.check_id) || '';
                if (data.source && data.source.log_pattern) {
                    const lp = data.source.log_pattern;
                    this.logPatternSeverities = lp.severities || ['warning', 'error', 'fatal'];
                    this.logPatternMinCount = lp.min_count || 10;
                    this.logPatternMaxAlertsPerApp = lp.max_alerts_per_app || 20;
                    this.logPatternEvaluateWithAI = lp.evaluate_with_ai !== false;
                }
                if (data.source && data.source.promql) {
                    this.promqlExpression = data.source.promql.expression || '';
                }
                this.selectorType = (data.selector && data.selector.type) || 'all';
                this.selectorCategories = (data.selector && data.selector.categories) || [];
                this.selectorPatternsText = ((data.selector && data.selector.application_id_patterns) || []).join(', ');
                this.severity = data.severity || 'warning';
                this.notificationCategory = data.notification_category || 'application';
                this.forDuration = this.$format.durationPretty(data['for'] || 0);
                this.keepFiringForDuration = this.$format.durationPretty(data['keep_firing_for'] || 0);
                this.templateSummary = (data.templates && data.templates.summary) || '';
                this.templateDescription = (data.templates && data.templates.description) || '';
                this.enabled = data.enabled !== false;
            });
        },
        buildPayload() {
            const isPromQL = this.sourceType === 'promql';
            return {
                name: this.name,
                source: {
                    type: this.sourceType,
                    check: this.sourceType === 'check' ? { check_id: this.checkId } : null,
                    log_pattern:
                        this.sourceType === 'log_patterns'
                            ? {
                                  severities: this.logPatternSeverities,
                                  min_count: this.logPatternMinCount,
                                  max_alerts_per_app: this.logPatternMaxAlertsPerApp,
                                  evaluate_with_ai: this.logPatternEvaluateWithAI,
                              }
                            : null,
                    promql: isPromQL ? { expression: this.promqlExpression } : null,
                },
                selector: {
                    type: isPromQL ? 'all' : this.selectorType,
                    categories: this.selectorType === 'category' && !isPromQL ? this.selectorCategories : [],
                    application_id_patterns:
                        this.selectorType === 'applications' && !isPromQL
                            ? this.selectorPatternsText
                                  .split(',')
                                  .map((p) => p.trim())
                                  .filter((p) => p)
                            : [],
                },
                severity: this.severity,
                notification_category: isPromQL ? this.notificationCategory : '',
                for: this.parseDuration(this.forDuration) || 0,
                keep_firing_for: this.parseDuration(this.keepFiringForDuration) || 0,
                templates: {
                    summary: isPromQL ? this.templateSummary : '',
                    description: this.templateDescription,
                },
                enabled: this.enabled,
            };
        },
        save() {
            if (!this.$refs.form.validate()) {
                return;
            }
            this.saving = true;
            this.error = '';
            const payload = this.buildPayload();
            if (this.ruleId) {
                this.$api.updateAlertingRule(this.ruleId, payload, (data, error) => {
                    this.saving = false;
                    if (error) {
                        this.error = error;
                        return;
                    }
                    this.$emit('saved');
                });
            } else {
                this.$api.createAlertingRule(payload, (data, error) => {
                    this.saving = false;
                    if (error) {
                        this.error = error;
                        return;
                    }
                    this.$emit('saved');
                });
            }
        },
        deleteRule() {
            this.saving = true;
            this.$api.deleteAlertingRule(this.ruleId, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$emit('saved');
            });
        },
        close() {
            this.dialog = false;
        },
    },
};
</script>

<style scoped>
.form-content {
    flex: 1;
    overflow-y: scroll;
    overflow-x: hidden;
    min-height: 0;
    padding: 20px;
    padding-top: 16px;
}
.form-content::-webkit-scrollbar {
    -webkit-appearance: none;
    width: 5px;
}
</style>
