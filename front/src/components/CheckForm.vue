<template>
    <v-dialog v-if="check" :value="value" @input="emitValue" max-width="800">
        <v-card class="pa-5">
            <div class="d-flex align-center font-weight-medium mb-4">
                <template v-if="check.id === 'SLOAvailability' || check.id === 'SLOLatency'">
                    Configure the "{{ check.title }}" inspection
                    <v-btn v-if="form && !form.default" small icon @click="confirmation = true"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                    <v-overlay :value="confirmation" absolute opacity="0.8">
                        <div>Are you sure you want to delete your override for the "{{ check.title }}" inspection?</div>
                        <div class="mt-5 d-flex">
                            <v-spacer />
                            <v-btn @click="confirmation = false" small color="info">Cancel</v-btn>
                            <v-btn @click="del" :loading="deleting" color="error" class="ml-3" small>Delete</v-btn>
                        </div>
                    </v-overlay>
                </template>
                <template v-else> Adjust the threshold for the "{{ check.title }}" inspection </template>
                <v-spacer />
                <v-btn icon @click="emitValue(false)"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <v-form v-if="form" v-model="valid">
                <CheckFormSLOAvailability v-if="check.id === 'SLOAvailability'" :form="form" />
                <CheckFormSLOLatency v-else-if="check.id === 'SLOLatency'" :form="form" />
                <CheckConfigForm v-else :form="form" :check="check" :appId="appId" />

                <div v-if="check.id.startsWith('SLO')" class="my-3">
                    Alerting:
                    <div>
                        <ul v-if="integrations && Object.keys(integrations).length">
                            <li v-for="(details, type) in integrations">
                                <span>{{ type }}</span>
                                <span v-if="details" class="grey--text"> ({{ details }})</span>
                            </li>
                        </ul>
                        <div v-else class="grey--text">No notification integrations configured.</div>
                        <v-btn color="primary" small :to="{ name: 'project_settings', params: { tab: 'notifications' } }" class="mt-1">
                            Configure integrations
                        </v-btn>
                    </div>
                </div>

                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="my-3">
                    {{ error }}
                </v-alert>
                <v-alert v-if="message" color="green" outlined text class="my-3">
                    {{ message }}
                </v-alert>
                <v-btn block color="primary" @click="save" :disabled="!(valid && changed)" :loading="saving" class="mt-5">Save</v-btn>
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script>
import CheckFormSLOAvailability from './CheckConfigSLOAvailabilityForm.vue';
import CheckFormSLOLatency from './CheckConfigSLOLatencyForm.vue';
import CheckConfigForm from './CheckConfigForm.vue';

export default {
    props: {
        appId: String,
        check: Object,
        value: Boolean,
    },

    components: { CheckConfigForm, CheckFormSLOAvailability, CheckFormSLOLatency },

    data() {
        return {
            loading: false,
            error: '',
            message: '',
            form: null,
            integrations: null,
            saved: '',
            saving: false,
            valid: false,
            deleting: false,
            confirmation: false,
        };
    },

    watch: {
        value() {
            if (this.value) {
                this.form = null;
                this.get();
            }
        },
    },

    computed: {
        changed() {
            return !!this.form && this.saved !== JSON.stringify(this.form);
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.$api.getInspectionConfig(this.appId, this.check.id, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    this.form = null;
                    return;
                }
                this.form = data.form;
                this.integrations = data.integrations;
                this.saved = JSON.stringify(this.form);
            });
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveInspectionConfig(this.appId, this.check.id, this.form, (data, error) => {
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
                this.get();
            });
        },
        del() {
            this.confirmation = false;
            this.deleting = true;
            this.error = '';
            this.$api.saveInspectionConfig(this.appId, this.check.id, { configs: null }, (data, error) => {
                this.deleting = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                this.emitValue(false);
            });
        },
        emitValue(v) {
            this.$emit('input', v);
        },
    },
};
</script>

<style scoped></style>
