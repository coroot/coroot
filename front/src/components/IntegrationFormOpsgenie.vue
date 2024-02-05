<template>
    <div>
        <div class="subtitle-1">To configure an <b>Rest HTTP API</b> integration in your Opsgenie account:</div>
        <ol class="mb-4 caption">
            <li>Navigate to the <b>Teams</b> page</li>
            <li>Choose a team (or create a new one)</li>
            <li>Go to <b>Integrations</b> &rarr; <b>Add integration</b> and create an <b>API</b> integration</li>
            <li>Change the <b>Name</b>, if necessary (e.g. <i>Coroot</i>)</li>
            <li>Adjust permissions (Coroot requires only <b>Create and Update Access</b>)</li>
            <li>Press <b>Save Integration</b></li>
            <li>Copy the <b>API Key</b> and paste it below</li>
        </ol>

        <div class="subtitle-1">API Key</div>
        <div class="d-flex">
            <!-- eslint-disable-next-line vue/no-mutating-props -->
            <v-text-field v-model="form.api_key" outlined dense :rules="[$validators.notEmpty]" />
            <!-- eslint-disable-next-line vue/no-mutating-props -->
            <v-checkbox v-model="form.eu_instance" label="EU instance of Opsgenie" dense hide-details class="ml-3" />
        </div>

        <div class="subtitle-1">Notify of</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.incidents" label="Incidents" dense hide-details />
        <v-checkbox :value="false" disabled label="Deployments (unavailable for Opsgenie integrations)" dense hide-details />
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
