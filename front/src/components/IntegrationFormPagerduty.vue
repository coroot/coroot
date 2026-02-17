<template>
    <div>
        <div class="subtitle-1">To configure an <b>Events API V2</b> integration in your Pagerduty account:</div>
        <ol class="mb-4 caption">
            <li>Navigate to <b>Services</b> &rarr; <b>Service Directory</b></li>
            <li>Choose a target service (or create a new one)</li>
            <li>Go to <b>Integrations</b> &rarr; <b>+ Add another integration</b> and create the <b>Events API V2</b> integration</li>
            <li>Change the <b>Integration Name</b>, if necessary (e.g. <i>Coroot</i>)</li>
            <li>Copy the <b>Integration Key</b> and paste it below</li>
        </ol>

        <div class="subtitle-1">Integration Key</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-text-field v-model="form.integration_key" outlined dense :rules="[$validators.notEmpty]" />

        <div class="subtitle-1">Notify of</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.incidents" label="Incidents" dense hide-details />
        <v-checkbox :value="false" disabled label="Deployments (unavailable for Pagerduty integrations)" dense hide-details />
    </div>
</template>

<script>
export default {
    props: {
        form: Object,
    },
};
</script>

<style scoped></style>
