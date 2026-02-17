<template>
    <div>
        <div class="subtitle-1">To configure an <b>Incoming webhook with Workflows</b> in your Microsoft Teams:</div>
        <ol class="mb-4 caption">
            <li>Choose a channel (or create a new one)</li>
            <li>Click <v-icon color="black">mdi-dots-horizontal</v-icon> next to the channel and select <b>Workflows</b></li>
            <li>Choose the <b>Post to a channel when a webhook request is received</b> workflow template</li>
            <li>Provide a name (e.g., <i>Coroot</i>) and click <b>Next</b></li>
            <li>Click <b>Add workflow</b></li>
            <li>Copy the webhook URL and paste it below</li>
        </ol>

        <div class="subtitle-1">Webhook URL</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-text-field v-model="form.webhook_url" outlined dense :rules="[$validators.notEmpty]" />

        <div class="subtitle-1">Notify of</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.incidents" label="Incidents" dense hide-details />
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.deployments" label="Deployments" dense hide-details />
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.alerts" label="Alerts" dense hide-details />
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
