<template>
    <div>
        <div class="subtitle-1">To configure an <b>Incoming webhook connector</b> in your Microsoft Teams:</div>
        <ol class="mb-4 caption">
            <li>Navigate to <b>Teams</b></li>
            <li>Choose a channel (or create a new one)</li>
            <li>Select <v-icon color="black">mdi-dots-horizontal</v-icon> from the top navigation menu and choose <b>Connectors</b></li>
            <li>Search for <b>Incoming Webhook</b> and press the <b>Configure</b> button</li>
            <li>
                Provide a name for the webhook (e.g. <i>Coroot</i>), customize the image (you can use the
                <a href="https://coroot.com/static/img/coroot.png" target="_blank">Coroot logo</a>), and click
                <b>Create</b>
            </li>
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
