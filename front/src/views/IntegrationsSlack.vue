<template>
    <v-dialog v-model="dialog" max-width="800">
        <v-card class="pa-4">
            <div class="d-flex align-center font-weight-medium mb-4">
                <div>
                    Configure Slack integration
                </div>
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <v-form v-model="valid" :disabled="deleting">
                <template v-if="!deleting">
                    <div class="subtitle-1">Slack app</div>
                    <div class="caption">
                        Click the button below to create your Slack App using the Coroot configuration. <br>
                        Once created, click “Install to workspace” to authorize it.
                    </div>
                    <v-btn :href="href" target="_blank" color="primary" class="mt-3 mb-5">
                        Create Slack app
                        <v-icon small class="ml-1">mdi-open-in-new</v-icon>
                    </v-btn>
                </template>

                <div class="subtitle-1">Slack Bot User OAuth Token</div>
                <div class="caption">
                    Click on "OAuth and Permissions" in the sidebar, copy the “Bot User OAuth Token” and paste it here.
                </div>
                <v-text-field v-model="form.token" outlined dense :rules="[$validators.notEmpty]"/>

                <div class="subtitle-1">Slack channel name</div>
                <div class="caption">
                    Open Slack, create a public channel and enter its name below.
                </div>
                <v-text-field v-model="form.channel" outlined dense :rules="[$validators.notEmpty]">
                    <template #prepend-inner><span class="grey--text mt-1">#</span></template>
                </v-text-field>

                <v-checkbox v-model="form.enabled" label="Enabled" class="mt-1" />

                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                    {{error}}
                </v-alert>
                <v-alert v-if="message" color="green" outlined text>
                    {{message}}
                </v-alert>
                <div class="d-flex align-center">
                    <v-spacer />
                    <v-btn v-if="deleting" @click="save" color="red" :loading="saving">Delete</v-btn>
                    <v-btn v-else @click="save" color="primary" :disabled="!valid" :loading="saving">Save</v-btn>
                </div>
            </v-form>
        </v-card>
    </v-dialog>
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
`

export default {
    props: {
        value: String,
    },

    data() {
        return {
            dialog: !!this.value,
            loading: false,
            error: '',
            message: '',
            saving: false,
            form: {
                token: '',
                channel: '',
                enabled: false,
            },
            valid: false,
        };
    },

    watch: {
        value(v) {
            this.dialog = !!v;
            if (v) {
                this.get();
            }
        },
        dialog(v) {
            this.$emit('input', v ? this.value : '');
        },
    },

    computed: {
        deleting() {
            return this.value === 'del';
        },
        href() {
            return 'https://api.slack.com/apps?new_app=1&manifest_yaml=' + encodeURIComponent(manifest);
        },
    },

    mounted() {
        this.get();
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getIntegrations('slack', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form = data;
            });
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations('slack', this.deleting ? null : this.form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                if (this.deleting) {
                    this.dialog = false;
                    return;
                }
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                }, 1000);
            });
        },
    },
}
</script>

<style scoped>

</style>