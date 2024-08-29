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
            Histogram query:
            <MetricSelector v-model="config.histogram_query" :rules="[$validators.notEmpty]" wrap="sum by(le)( rate( <input> [..]) )" class="mb-3" />
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
                class="input text"
            >
                <template #append><span class="grey--text">%</span></template>
            </v-text-field>
            of requests should be served faster than
            <v-select
                :disabled="!trackSLO"
                v-model.number="config.objective_bucket"
                :items="buckets"
                :rules="[$validators.notEmpty]"
                outlined
                dense
                hide-details
                :menu-props="{ offsetY: true }"
                class="input select"
            />
        </div>
    </div>
</template>

<script>
import MetricSelector from './MetricSelector';

const buckets = [
    { value: 0.005, text: '5ms' },
    { value: 0.01, text: '10ms' },
    { value: 0.025, text: '25ms' },
    { value: 0.05, text: '50ms' },
    { value: 0.1, text: '100ms' },
    { value: 0.25, text: '250ms' },
    { value: 0.5, text: '500ms' },
    { value: 1, text: '1s' },
    { value: 2.5, text: '2.5s' },
    { value: 5, text: '5s' },
    { value: 10, text: '10s' },
];
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
    computed: {
        config() {
            return this.form.configs[0];
        },
        buckets() {
            return buckets;
        },
    },
    methods: {
        changeTrackSLO() {
            this.config.objective_percentage = this.trackSLO ? 99 : 0;
        },
    },
};
</script>

<style scoped>
.input {
    display: inline-flex;
}
.input >>> .v-input__slot {
    min-height: initial !important;
    height: 1.5rem !important;
    padding: 0 8px !important;
}
.input.text >>> .v-input__append-inner {
    margin-top: 4px !important;
}
.input.select >>> .v-input__append-inner {
    margin-top: 0 !important;
}
.input >>> .v-select__selection--comma {
    margin: 0 !important;
}
* >>> .v-list-item {
    min-height: 32px !important;
    padding: 0 8px !important;
}
.input.text {
    max-width: 7ch;
}
.input.select {
    max-width: 11ch;
}
</style>
