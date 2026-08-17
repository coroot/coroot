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

        <div class="subtitle-1">Channels</div>
        <div class="caption mb-2">
            Notifications are routed to a channel by name in the application category settings. The default channel is used unless a category
            specifies another one.
        </div>
        <div v-for="(ch, i) in form.channels" :key="i" class="d-flex mb-2 align-center" style="gap: 8px">
            <v-text-field
                outlined
                dense
                v-model="ch.name"
                label="channel name"
                hide-details
                single-line
                :rules="[$validators.notEmpty]"
                class="name"
            />
            <v-text-field
                outlined
                dense
                v-model="ch.webhook_url"
                label="webhook url"
                hide-details
                single-line
                :rules="[$validators.notEmpty, $validators.isUrl]"
            />
            <!-- eslint-disable-next-line vue/no-mutating-props -->
            <v-btn @click="form.channels.splice(i, 1)" icon small>
                <v-icon small>mdi-trash-can-outline</v-icon>
            </v-btn>
        </div>
        <div v-if="!hasChannels" class="caption grey--text mb-2">No channels configured. Saving now will remove the integration.</div>
        <v-btn color="primary" small @click="addChannel">Add channel</v-btn>

        <template v-if="hasChannels">
            <div class="subtitle-1 mt-5">Default channel</div>
            <!-- eslint-disable vue/no-mutating-props -->
            <v-select
                v-model="form.default_channel"
                :items="channelNames"
                outlined
                dense
                :menu-props="{ offsetY: true }"
                :rules="[$validators.notEmpty, isConfiguredChannel]"
            />
            <!-- eslint-enable vue/no-mutating-props -->
        </template>

        <div class="subtitle-1" :class="{ 'mt-5': !hasChannels }">Notify of</div>
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

    computed: {
        hasChannels() {
            return !!(this.form.channels && this.form.channels.length);
        },
        channelNames() {
            return (this.form.channels || []).map((ch) => ch.name).filter((n) => !!n);
        },
    },

    methods: {
        isConfiguredChannel(v) {
            return this.channelNames.includes(v) || 'must be one of the configured channels';
        },
        addChannel() {
            if (!this.form.channels) {
                // eslint-disable-next-line vue/no-mutating-props
                this.$set(this.form, 'channels', []);
            }
            // eslint-disable-next-line vue/no-mutating-props
            this.form.channels.push({ name: '', webhook_url: '' });
        },
    },
};
</script>

<style scoped>
.name {
    max-width: 30%;
}
</style>
