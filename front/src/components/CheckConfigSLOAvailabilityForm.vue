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
        <div class="d-flex" style="gap: 4px">
            <v-checkbox v-model="trackSLO" @change="changeTrackSLO" hide-details class="mt-0 pt-0" />
            <v-text-field
                :disabled="!trackSLO"
                outlined
                dense
                v-model.number="config.objective_percentage"
                :rules="[$validators.isFloat]"
                hide-details
                class="input"
            >
                <template #append><span class="grey--text">%</span></template>
            </v-text-field>
            of requests should not fail
        </div>
    </div>
</template>

<script>
import MetricSelector from './MetricSelector';

export default {
    components: { MetricSelector },
    props: {
        form: Object,
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
    },
};
</script>

<style scoped>
.input {
    display: inline-flex;
    max-width: 8ch;
}
.input >>> .v-input__slot {
    min-height: initial !important;
    height: 1.5rem !important;
    padding: 0 8px !important;
}
.input >>> .v-input__append-inner {
    margin-top: 4px !important;
}
</style>
