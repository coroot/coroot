<template>
    <v-dialog v-model="dialog" max-width="800">
        <v-card class="pa-5">
            <div class="d-flex align-center font-weight-medium mb-4">
                <div>
                    Configure {{ title }} integration
                    <a :href="`https://docs.coroot.com/alerting/${type}`" target="_blank">
                        <v-icon>mdi-information-outline</v-icon>
                    </a>
                    <v-progress-circular v-if="loading" indeterminate color="green" size="30" />
                </div>
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <v-form ref="form" v-model="valid" :disabled="value === 'del'">
                <IntegrationFormSlack v-if="type === 'slack'" :form="form" />
                <IntegrationFormTeams v-if="type === 'teams'" :form="form" />
                <IntegrationFormPagerduty v-if="type === 'pagerduty'" :form="form" />
                <IntegrationFormOpsgenie v-if="type === 'opsgenie'" :form="form" />
                <IntegrationFormWebhook v-if="type === 'webhook'" :form="form" />

                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="my-4">
                    {{ error }}
                </v-alert>
                <v-alert v-if="message" color="green" outlined text class="my-4">
                    {{ message }}
                </v-alert>
                <div class="d-flex align-center">
                    <v-spacer />
                    <v-btn v-if="value === 'del'" @click="del" color="red" :loading="saving">Delete</v-btn>
                    <template v-else>
                        <v-btn @click="test" color="accent" :disabled="!valid" :loading="testing" class="mr-4">Send test alert</v-btn>
                        <v-btn @click="save" color="primary" :disabled="!valid" :loading="saving">Save</v-btn>
                    </template>
                </div>
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script>
import IntegrationFormSlack from '../components/IntegrationFormSlack.vue';
import IntegrationFormTeams from '../components/IntegrationFormTeams.vue';
import IntegrationFormPagerduty from '../components/IntegrationFormPagerduty.vue';
import IntegrationFormOpsgenie from '../components/IntegrationFormOpsgenie.vue';
import IntegrationFormWebhook from '../components/IntegrationFormWebhook.vue';

export default {
    props: {
        value: String,
        type: String,
        title: String,
    },

    components: { IntegrationFormSlack, IntegrationFormTeams, IntegrationFormPagerduty, IntegrationFormOpsgenie, IntegrationFormWebhook },

    data() {
        return {
            dialog: !!this.value,
            loading: false,
            error: '',
            message: '',
            saving: false,
            testing: false,
            valid: false,
            form: {},
        };
    },

    watch: {
        dialog(v) {
            this.$emit('input', v ? this.value : '');
        },
    },

    mounted() {
        this.get();
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getIntegrations(this.type, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                if (this.value === 'new') {
                    this.$refs.form && this.$refs.form.resetValidation();
                }
                this.form = data;
            });
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations(this.type, 'save', this.form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                }, 1000);
            });
        },
        del() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations(this.type, 'del', null, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                this.dialog = false;
            });
        },
        test() {
            this.testing = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations(this.type, 'test', this.form, (data, error) => {
                this.testing = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'A test alert has been successfully sent.';
                setTimeout(() => {
                    this.message = '';
                }, 3000);
            });
        },
    },
};
</script>

<style scoped></style>
