<template>
    <div>
        Metrics:
        <v-select v-model="config.custom" :items="[{value: false, text: 'inbound requests (built-in)'}, {value: true, text: 'custom'}]" outlined dense :menu-props="{offsetY: true}" hide-details class="mb-3" />

        <template v-if="config.custom">
            Total requests query:
            <MetricSelector v-model="config.total_requests_query" :rules="config.custom ? [$validators.notEmpty] : []" wrap="sum( rate( <input> [..]) )" class="mb-3"/>

            Failed requests query:
            <MetricSelector v-model="config.failed_requests_query" :rules="config.custom ? [$validators.notEmpty] : []" wrap="sum( rate( <input> [..]) )" class="mb-3"/>
        </template>

        Objective:
        <div>
            <v-text-field outlined dense v-model.number="config.objective_percentage" :rules="[$validators.isFloat]" hide-details class="input">
                <template #append><span class="grey--text">%</span></template>
            </v-text-field>
            of requests should not fail
        </div>
    </div>
</template>

<script>
import MetricSelector from "./MetricSelector";

export default {
    components: {MetricSelector},
    props: {
        form: Object,
    },
    computed: {
        config() {
            return this.form.configs[0];
        },
    },
}
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