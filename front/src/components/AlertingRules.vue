<template>
    <div>
        <div class="d-flex align-center mb-4">
            <div>
                <v-text-field v-model="search" label="search" prepend-inner-icon="mdi-magnify" dense outlined hide-details clearable class="search" />
            </div>
            <v-spacer />
            <v-btn color="primary" small @click="openForm('new')" class="mr-4">
                <v-icon small class="mr-1">mdi-plus</v-icon>
                Add rule
            </v-btn>
            <v-btn outlined small @click="showExport" class="mr-4">
                <v-icon small class="mr-1">mdi-export</v-icon>
                Export
            </v-btn>
            <v-menu offset-y max-width="500" :close-on-content-click="false">
                <template #activator="{ on }">
                    <v-icon v-on="on" color="primary" title="What are alerting rules?">mdi-help-circle-outline</v-icon>
                </template>
                <v-card class="pa-3">
                    <div class="body-2">
                        Alerting rules define conditions that trigger notifications to your configured channels. Conditions can be based on inspection
                        results, PromQL expressions, or log patterns. Each rule specifies the severity level and which applications to monitor.
                    </div>
                </v-card>
            </v-menu>
        </div>

        <div v-if="selected.length" class="d-flex align-center justify-end mb-3">
            <span class="caption red--text mr-2">Disabling selected rules will resolve their firing alerts</span>
            <v-btn small outlined class="mr-2" @click="bulkSetEnabled(true)"> Enable ({{ selected.length }}) </v-btn>
            <v-btn small outlined @click="bulkSetEnabled(false)"> Disable ({{ selected.length }}) </v-btn>
        </div>

        <v-data-table
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="50"
            :items="filteredRules"
            sort-by="name"
            no-data-text="No alerting rules found"
            :headers="[
                { value: 'select', text: '', sortable: false, width: '40px' },
                { value: 'name', text: 'Name' },
                { value: 'type', text: 'Data source' },
                { value: 'selector', text: 'Application selector' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100] }"
        >
            <template #header.select>
                <v-checkbox
                    :value="allSelected"
                    :indeterminate="someSelected && !allSelected"
                    @change="toggleSelectAll"
                    hide-details
                    class="mt-0 pt-0"
                    color="primary"
                />
            </template>

            <template #item.select="{ item }">
                <v-checkbox
                    v-if="!item.readonly"
                    :value="selected.includes(item.id)"
                    @change="toggleSelect(item.id)"
                    hide-details
                    class="mt-0 pt-0"
                    color="primary"
                />
            </template>

            <template #item.name="{ item }">
                <div class="rule-item" :class="{ 'grey--text': !item.enabled }">
                    <div class="severity-bar" :class="severityColor(item.severity)" :title="severityLabel(item.severity)" />
                    <a @click="openForm(item.id)" class="name-link" :class="{ disabled: !item.enabled }">{{ item.name }}</a>
                    <v-icon v-if="item.readonly" small class="ml-1" title="Managed via config">mdi-lock-outline</v-icon>
                    <v-icon v-if="!item.enabled" small class="ml-1" title="Disabled">mdi-bell-off-outline</v-icon>
                    <router-link
                        v-if="alertCounts[item.id]"
                        :to="{ name: 'overview', params: { view: 'alerts' }, query: { ...$utils.contextQuery(), search: item.name } }"
                        class="ml-2"
                        @click.native.stop
                    >
                        <v-chip label small style="cursor: pointer">
                            {{ alertCounts[item.id] }} active {{ alertCounts[item.id] === 1 ? 'alert' : 'alerts' }}
                        </v-chip>
                    </router-link>
                </div>
            </template>

            <template #item.type="{ item }">
                <span v-if="item.source && item.source.type === 'check'">
                    Inspection:
                    <router-link
                        :to="{
                            params: { id: 'inspections' },
                            query: { ...$utils.contextQuery(), search: getCheckTitle(item.source.check.check_id) },
                        }"
                    >
                        {{ getCheckTitle(item.source.check.check_id) }}
                    </router-link>
                </span>
                <span v-else-if="item.source && item.source.type === 'log_patterns'">Log patterns</span>
                <span v-else-if="item.source && item.source.type === 'promql'">
                    PromQL: <code>{{ truncateExpr(item.source.promql && item.source.promql.expression) }}</code>
                </span>
                <span v-else>-</span>
            </template>

            <template #item.selector="{ item }">
                <span v-if="!item.selector || item.selector.type === 'all'">All</span>
                <span v-else-if="item.selector.type === 'category'">{{ (item.selector.categories || []).join(', ') }}</span>
                <span v-else-if="item.selector.type === 'applications'">{{ (item.selector.application_id_patterns || []).join(', ') }}</span>
                <span v-else>-</span>
            </template>
        </v-data-table>

        <v-dialog v-model="exportDialog" width="80%" scrollable>
            <v-card class="pa-5">
                <div class="d-flex align-center mb-4">
                    <div class="font-weight-medium">Export alerting rules</div>
                    <v-spacer />
                    <v-btn icon @click="exportDialog = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <div class="caption grey--text mb-2">Paste this snippet into your project configuration file.</div>
                <div class="code-block">
                    <v-btn icon x-small class="copy-btn" @click="copyExport" :title="copied ? 'Copied' : 'Copy'">
                        <v-icon small>{{ copied ? 'mdi-check' : 'mdi-content-copy' }}</v-icon>
                    </v-btn>
                    <pre v-if="exportYaml">{{ exportYaml }}</pre>
                    <v-progress-linear v-else indeterminate />
                </div>
            </v-card>
        </v-dialog>

        <AlertingRuleForm
            v-if="ruleId"
            :key="ruleId"
            :rule-id="ruleId === 'new' ? null : ruleId"
            :checks="checks"
            :categories="categories"
            @close="closeForm"
            @saved="onRuleSaved"
        />
    </div>
</template>

<script>
import AlertingRuleForm from './AlertingRuleForm.vue';

export default {
    components: { AlertingRuleForm },

    data() {
        return {
            rules: [],
            checks: [],
            categories: [],
            alertCounts: {},
            search: '',
            selected: [],
            exportDialog: false,
            exportYaml: '',
            copied: false,
        };
    },

    computed: {
        ruleId() {
            return this.$route.query.rule;
        },
        filteredRules() {
            if (!this.search) {
                return this.rules;
            }
            const s = this.search.toLowerCase();
            return this.rules.filter((r) => {
                const name = (r.name || '').toLowerCase();
                const checkId = (r.source?.check?.check_id || '').toLowerCase();
                const categories = (r.selector?.categories || []).join(' ').toLowerCase();
                const patterns = (r.selector?.application_id_patterns || []).join(' ').toLowerCase();
                const severity = (r.severity || '').toLowerCase();
                return name.includes(s) || checkId.includes(s) || categories.includes(s) || patterns.includes(s) || severity.includes(s);
            });
        },
        selectableRules() {
            return this.filteredRules.filter((r) => !r.readonly);
        },
        allSelected() {
            return this.selectableRules.length > 0 && this.selected.length === this.selectableRules.length;
        },
        someSelected() {
            return this.selected.length > 0;
        },
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    methods: {
        get() {
            this.$emit('loading', true);
            this.$api.getAlertingRules((data, error) => {
                this.$emit('loading', false);
                if (error) {
                    this.$emit('error', error);
                    return;
                }
                this.rules = (data && data.rules) || [];
                this.checks = (data && data.checks) || [];
                this.categories = (data && data.categories) || [];
                this.alertCounts = (data && data.alert_counts) || {};
            });
        },
        openForm(ruleId) {
            this.$router.push({ query: { ...this.$route.query, rule: ruleId } }).catch(() => {});
        },
        closeForm() {
            this.$router.replace({ query: { ...this.$route.query, rule: undefined } });
        },
        onRuleSaved() {
            this.closeForm();
            this.get();
        },
        toggleSelect(id) {
            const index = this.selected.indexOf(id);
            if (index >= 0) {
                this.selected.splice(index, 1);
            } else {
                this.selected.push(id);
            }
        },
        toggleSelectAll() {
            if (this.allSelected) {
                this.selected = [];
            } else {
                this.selected = this.selectableRules.map((r) => r.id);
            }
        },
        async bulkSetEnabled(enabled) {
            const ids = [...this.selected];
            for (const id of ids) {
                const rule = this.rules.find((r) => r.id === id);
                if (rule) {
                    await new Promise((resolve) => {
                        this.$api.updateAlertingRule(id, { ...rule, enabled }, () => {
                            resolve();
                        });
                    });
                }
            }
            this.selected = [];
            this.get();
        },
        severityLabel(severity) {
            if (severity === 'critical') return 'Critical';
            if (severity === 'warning') return 'Warning';
            return 'Warning';
        },
        severityColor(severity) {
            if (severity === 'critical') return 'red lighten-1';
            if (severity === 'warning') return 'orange lighten-1';
            return 'orange lighten-1';
        },
        getCheckTitle(checkId) {
            const check = this.checks.find((c) => c.id === checkId);
            return check ? check.title : checkId;
        },
        truncateExpr(expr) {
            if (!expr) return '';
            return expr.length > 60 ? expr.substring(0, 57) + '...' : expr;
        },
        showExport() {
            this.exportYaml = '';
            this.copied = false;
            this.exportDialog = true;
            this.$api.get(this.$api.projectPath('alerting-rules/export'), {}, (data, error) => {
                if (error) {
                    this.exportYaml = '# Error: ' + error;
                    return;
                }
                this.exportYaml = (data && data.yaml) || '';
            });
        },
        copyExport() {
            navigator.clipboard.writeText(this.exportYaml).then(() => {
                this.copied = true;
                setTimeout(() => {
                    this.copied = false;
                }, 2000);
            });
        },
    },
};
</script>

<style scoped>
.table:deep(table) {
    min-width: 500px;
}
.table:deep(tr:hover) {
    background-color: unset !important;
}
.table:deep(th),
.table:deep(td) {
    padding: 4px 8px !important;
}
.table:deep(th) {
    white-space: nowrap;
}
.table:deep(td:has(.rule-item)) {
    padding-left: 0 !important;
}
.rule-item {
    display: flex;
    align-items: center;
    gap: 4px;
}
.rule-item .severity-bar {
    width: 4px;
    height: 20px;
}
.name-link {
    cursor: pointer;
    color: #1976d2;
    text-decoration: underline;
}
.name-link.disabled {
    color: grey;
}

.table:deep(.v-input--checkbox .v-icon) {
    color: rgba(0, 0, 0, 0.25) !important;
    font-size: 18px !important;
}
.table:deep(.v-input--checkbox.v-input--is-label-active .v-icon) {
    color: rgba(0, 0, 0, 0.7) !important;
}
.table:deep(.v-input--checkbox .v-input--selection-controls__ripple) {
    display: none;
}
.search:deep(input) {
    width: 200px !important;
}
.code-block {
    position: relative;
    background-color: var(--background-color-hi, #f5f5f5);
    border-radius: 4px;
    padding: 12px;
    max-height: 70vh;
    overflow: auto;
}
.code-block pre {
    margin: 0;
    font-size: 13px;
    line-height: 1.5;
    white-space: pre;
}
.code-block .copy-btn {
    position: absolute;
    top: 8px;
    right: 8px;
    opacity: 0.5;
}
.code-block .copy-btn:hover {
    opacity: 1;
}
</style>
