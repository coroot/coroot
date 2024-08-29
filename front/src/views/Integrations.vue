<template>
    <div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{ message }}
        </v-alert>
        <v-form>
            <div class="subtitle-1">Base url</div>
            <div class="caption">This URL is used for things like creating links in alerts.</div>
            <div class="d-flex">
                <v-text-field v-model="form.base_url" :rules="[$validators.isUrl]" outlined dense />
                <v-btn @click="save" color="primary" :loading="saving" class="ml-2" height="38">Save</v-btn>
            </div>
        </v-form>

        <v-simple-table>
            <thead>
                <tr>
                    <th>Type</th>
                    <th>Notify of incidents</th>
                    <th>Notify of deployments</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="i in integrations">
                    <td>
                        {{ i.title }}
                        <div class="caption">{{ i.details }}</div>
                    </td>
                    <td>
                        <v-icon v-if="i.configured" small :color="i.incidents ? 'green' : ''">
                            {{ i.incidents ? 'mdi-check' : 'mdi-minus' }}
                        </v-icon>
                    </td>
                    <td>
                        <v-icon v-if="i.configured" small :color="i.deployments ? 'green' : ''">
                            {{ i.deployments ? 'mdi-check' : 'mdi-minus' }}
                        </v-icon>
                    </td>
                    <td>
                        <v-btn v-if="!i.configured" small @click="open(i, 'new')" color="primary">Configure</v-btn>
                        <div v-else class="d-flex">
                            <v-btn icon small @click="open(i, 'edit')"><v-icon small>mdi-pencil</v-icon></v-btn>
                            <v-btn icon small @click="open(i, 'del')"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                        </div>
                    </td>
                </tr>
            </tbody>
        </v-simple-table>

        <IntegrationForm v-if="action" v-model="action" :type="integration.type" :title="integration.title" />
    </div>
</template>

<script>
import IntegrationForm from './IntegrationForm.vue';

export default {
    props: {
        projectId: String,
    },

    components: { IntegrationForm },

    data() {
        return {
            loading: false,
            error: '',
            message: '',
            saving: false,
            form: {
                base_url: '',
            },
            integrations: [],
            integration: {},
            action: '',
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        projectId() {
            this.get();
        },
    },

    methods: {
        open(i, action) {
            this.integration = i;
            this.action = action;
        },
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getIntegrations('', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form.base_url = data.base_url;
                if (!this.form.base_url) {
                    this.form.base_url = location.origin + this.$coroot.base_path;
                    this.$api.saveIntegrations('', 'save', this.form, () => {});
                }
                this.integrations = data.integrations;
            });
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations('', 'save', this.form, (data, error) => {
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
};
</script>

<style scoped></style>
