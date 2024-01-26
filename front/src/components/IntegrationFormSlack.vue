<template>
    <div>
        <div class="subtitle-1">Slack app</div>
        <div class="caption">
            Click the button below to create your Slack App using the Coroot configuration. <br />
            Once created, click <b>Install to workspace</b> to authorize it.
        </div>
        <v-btn :href="href" target="_blank" color="primary" class="mt-3 mb-5">
            Create Slack app
            <v-icon small class="ml-1">mdi-open-in-new</v-icon>
        </v-btn>

        <div class="subtitle-1">Slack app icon</div>
        <div class="caption mb-4">
            Customize the image (you can use the <a href="https://coroot.com/static/img/coroot_512.png" target="_blank">Coroot logo</a>)
        </div>

        <div class="subtitle-1">Slack Bot User OAuth Token</div>
        <div class="caption">Click on <b>OAuth and Permissions</b> in the sidebar, copy the <b>Bot User OAuth Token</b> and paste it here.</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-text-field v-model="form.token" outlined dense :rules="[$validators.notEmpty]" />

        <div class="subtitle-1">Slack channel name</div>
        <div class="caption">Open Slack, create a public channel and enter its name below.</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-text-field v-model="form.default_channel" outlined dense :rules="[$validators.notEmpty]">
            <template #prepend-inner><span class="grey--text mt-1">#</span></template>
        </v-text-field>

        <div class="subtitle-1">Notify of</div>
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.incidents" label="Incidents" dense hide-details />
        <!-- eslint-disable-next-line vue/no-mutating-props -->
        <v-checkbox v-model="form.deployments" label="Deployments" dense hide-details />
    </div>
</template>

<script>
const manifest = `
display_information:
  name: Coroot
  description: Track SLOs of your services
features:
  bot_user:
    display_name: Coroot
oauth_config:
  scopes:
    bot:
      - channels:read
      - chat:write
      - chat:write.public
`;

export default {
    props: {
        form: Object,
    },
    computed: {
        href() {
            return 'https://api.slack.com/apps?new_app=1&manifest_yaml=' + encodeURIComponent(manifest);
        },
    },
};
</script>

<style scoped></style>
