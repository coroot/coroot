<template>
    <v-dialog v-model="dialog" persistent no-click-animation max-width="80%">
        <v-card class="pa-4">
            <div class="d-flex align-center font-weight-medium mb-2 text-h5">
                <div class="text-capitalize">{{ action }} panel</div>
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <v-form v-model="valid">
                <v-row dense>
                    <v-col cols="8">
                        <div class="subtitle-1">Name</div>
                        <v-text-field v-model="config.name" :rules="[$validators.notEmpty]" outlined dense hide-details />
                    </v-col>
                    <v-col>
                        <div class="subtitle-1">Group</div>
                        <v-combobox
                            v-model="panel.group"
                            :items="groups_"
                            :search-input.sync="search"
                            :rules="[$validators.notEmpty]"
                            outlined
                            dense
                            hide-details
                            :menu-props="{ offsetY: true }"
                            :return-object="false"
                        />
                    </v-col>
                </v-row>
                <v-row dense class="mt-2">
                    <v-col cols="8">
                        <div class="subtitle-1">Description</div>
                        <v-text-field v-model="config.description" outlined dense hide-details />
                    </v-col>
                    <v-col>
                        <div class="subtitle-1">Type</div>
                        <v-select :value="'Time series chart'" :items="['Time series chart']" outlined dense hide-details disabled />
                    </v-col>
                </v-row>

                <div class="subtitle-1 mt-3">Preview</div>
                <Panel :config="config" style="height: 240px" />

                <div v-for="(_, i) in config.source.metrics.queries" class="mb-6">
                    <div class="subtitle-1 mt-2">Query #{{ i + 1 }}</div>

                    <div v-if="$api.context.multicluster" class="mb-3">
                        <div class="subtitle-1">Data Source</div>
                        <div class="caption">Select which cluster/project to query.</div>
                        <v-select
                            v-model="config.source.metrics.queries[i].datasource"
                            :items="datasources"
                            :rules="[$validators.notEmpty]"
                            outlined
                            dense
                            hide-details
                            placeholder="Select data source"
                        />
                    </div>

                    <div class="subtitle-1 mt-2">PromQL Query</div>
                    <div class="caption">PromQL expression.</div>
                    <MetricSelector v-model="config.source.metrics.queries[i].query" :datasource="config.source.metrics.queries[i].datasource" />

                    <div class="subtitle-1 mt-2">Legend</div>
                    <div class="caption">
                        Text to be displayed in the legend and the tooltip. Use <var v-pre>{{ label_name }}</var> to interpolate label values.
                    </div>
                    <v-text-field v-model="config.source.metrics.queries[i].legend" outlined dense hide-details />
                </div>
                <v-btn color="primary" @click="addQuery()">
                    <v-icon>mdi-plus</v-icon>
                    Add query
                </v-btn>

                <div class="d-flex align-center gap-2 mt-4">
                    <div class="subtitle-1" style="min-width: 100px">Stack series</div>
                    <v-checkbox v-model="config.widget.chart.stacked" dense hide-details class="mt-0 pt-0" />
                </div>
                <div class="d-flex align-center gap-2 mt-2">
                    <div class="subtitle-1" style="min-width: 100px">Display</div>
                    <v-btn-toggle v-model="config.widget.chart.display" dense mandatory>
                        <v-btn value="line">Line</v-btn>
                        <v-btn value="bar">Bar</v-btn>
                    </v-btn-toggle>
                </div>
            </v-form>
            <div class="d-flex gap-1">
                <v-spacer />
                <v-btn color="primary" @click="apply" :disabled="!valid">Apply</v-btn>
                <v-btn color="primary" outlined @click="dialog = false">Cancel</v-btn>
            </div>
        </v-card>
    </v-dialog>
</template>

<script>
import MetricSelector from '@/components/MetricSelector.vue';
import Panel from '@/views/dashboards/Panel.vue';

export default {
    props: {
        value: Object,
        groups: Array,
    },

    components: { Panel, MetricSelector },

    data() {
        const panel = JSON.parse(JSON.stringify(this.value));
        let action = 'edit';
        if (!panel.config) {
            action = 'add';
            panel.config = {
                name: '',
                description: '',
                source: { metrics: { queries: [{ query: '', legend: '', color: '', datasource: '' }] } },
                widget: { chart: {} },
            };
        }
        return {
            loading: false,
            error: '',
            dialog: !!this.value,
            action,
            panel,
            valid: false,
            search: '',
        };
    },

    mounted() {
        if (this.panel.config.source.metrics.queries[0].datasource === '' && this.$api.context.multicluster && this.datasources.length > 0) {
            this.panel.config.source.metrics.queries[0].datasource = this.datasources[0];
        }
    },

    watch: {
        dialog(v) {
            !v && this.$emit('input', null);
        },
    },

    computed: {
        datasources() {
            return this.$api.context.member_projects;
        },
        groups_() {
            const groups = this.groups.map((g) => ({ value: g, text: g }));
            if (!this.search || this.groups.includes(this.search)) {
                return groups;
            }
            return [{ value: this.search, text: this.search + ' (add new)' }, ...this.groups];
        },
        config() {
            return this.panel.config;
        },
    },

    methods: {
        addQuery() {
            const newQuery = { query: '', legend: '', color: '', datasource: '' };
            if (this.$api.context.multicluster && this.datasources.length > 0) {
                newQuery.datasource = this.datasources[0];
            }
            this.config.source.metrics.queries.push(newQuery);
        },

        apply() {
            this.dialog = false;
            this.$emit(this.action, JSON.parse(JSON.stringify(this.panel)));
        },
    },
};
</script>

<style scoped></style>
