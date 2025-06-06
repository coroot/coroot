<template>
    <div>
        Metrics:
        <v-select
            v-model="config.custom"
            :items="[
                { value: false, text: 'inbound requests (built-in)' },
                { value: true, text: 'custom' },
            ]"
            outlined
            dense
            :menu-props="{ offsetY: true }"
            :disabled="readonly || appId === '::'"
            hide-details
            class="mb-3"
        />

        <template v-if="config.custom">
            Total requests query:
            <MetricSelector
                v-model="config.total_requests_query"
                :rules="config.custom ? [$validators.notEmpty] : []"
                wrap="sum( rate( <input> [..]) )"
                class="mb-3"
            />

            Failed requests query:
            <MetricSelector
                v-model="config.failed_requests_query"
                :rules="config.custom ? [$validators.notEmpty] : []"
                wrap="sum( rate( <input> [..]) )"
                class="mb-3"
            />
        </template>

        Objective:
        <v-alert v-if="config.error" color="error" outlined text class="mt-1 mb-3 pa-2">
            {{ config.error }}
        </v-alert>
        <v-alert v-else-if="config.source === 'kubernetes-annotations'" color="info" outlined text class="mt-1 mb-3 pa-2">
            This SLO is configured via Kubernetes annotations.
        </v-alert>
        <v-form :disabled="readonly" class="d-flex gap-1">
            <v-checkbox v-model="trackSLO" @change="changeTrackSLO" hide-details class="mt-0 pt-0 checkbox" />
            <v-form :disabled="readonly || !trackSLO">
                <v-text-field outlined dense v-model.number="config.objective_percentage" :rules="[$validators.isFloat]" hide-details class="input">
                    <template #append><span class="grey--text">%</span></template>
                </v-text-field>
                of requests should not fail
            </v-form>
        </v-form>
    </div>
</template>

<script>
import MetricSelector from './MetricSelector';

export default {
    components: { MetricSelector },
    props: {
        form: Object,
        appId: String,
    },
    data() {
        return {
            trackSLO: this.form.configs[0].objective_percentage > 0,
        };
    },
    methods: {
        changeTrackSLO() {
            this.config.objective_percentage = this.trackSLO ? 99 : 0;
        },
    },
    computed: {
        config() {
            return this.form.configs[0];
        },
        readonly() {
            return !!this.config.source;
        },
    },
};
</script>

<style scoped>
.checkbox:deep(.v-input--selection-controls__input) {
    margin-right: 0 !important;
}
.input {
    display: inline-flex;
    max-width: 8ch;
}
.input:deep(.v-input__slot) {
    min-height: initial !important;
    height: 1.5rem !important;
    padding: 0 8px !important;
}
.input:deep(.v-input__append-inner) {
    margin-top: 4px !important;
}
</style>
