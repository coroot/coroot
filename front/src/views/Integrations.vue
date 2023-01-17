<template>
<div>
    <v-form>
        <div class="subtitle-1">Base url</div>
        <div class="caption">
            This URL is used for things like creating links in alerts.
        </div>

        <div class="d-flex">
            <v-text-field v-model="form.base_url" :rules="[$validators.isUrl]" outlined dense />
            <v-btn @click="save" color="primary" :loading="saving" class="ml-2" height="38">Save</v-btn>
        </div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{error}}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{message}}
        </v-alert>
    </v-form>

    <v-simple-table>
        <thead>
        <tr>
            <th>Type</th>
            <th>Details</th>
            <th>Actions</th>
        </tr>
        </thead>
        <tbody>
        <tr>
            <td>Slack</td>
            <td>
                <span v-if="slack.info">
                    channel: #{{slack.info.channel}},
                    enabled: {{slack.info.enabled}},
                    available: {{slack.info.available}}
                </span>
                <span v-else class="grey--text">not configured</span>
            </td>
            <td>
                <v-btn v-if="!slack.info" small @click="slack.action = 'add'" color="primary">Configure</v-btn>
                <div v-else class="d-flex">
                    <v-btn icon small @click="slack.action='edit'"><v-icon small>mdi-pencil</v-icon></v-btn>
                    <v-btn icon small @click="slack.action='del'"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                </div>
            </td>
        </tr>
        </tbody>
    </v-simple-table>
    <IntegrationsSlack v-model="slack.action" />
</div>
</template>

<script>
import IntegrationsSlack from "@/views/IntegrationsSlack";

export default {
    props: {
        projectId: String,
    },

    components: {IntegrationsSlack},

    data() {
        return {
            loading: false,
            error: '',
            message: '',
            saving: false,
            form: {
                base_url: '',
            },
            slack: {
                action: '',
                info: null
            },
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        projectId() {
            this.get();
        }
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getIntegrations('',(data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form.base_url = data.base_url;
                if (!this.form.base_url) {
                    this.form.base_url = location.origin + this.$coroot.base_path;
                    this.$api.saveIntegrations('', this.form, () => {});
                }
                this.slack.info = data.slack;
            });
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations('', this.form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                }, 1000);
                this.get();
            });
        },
    },
}
</script>

<style scoped>

</style>