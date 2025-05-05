<template>
    <v-dialog v-model="dialog" max-width="800">
        <template #activator="{ on }">
            <a v-on="on"><b>Configure</b></a>
        </template>

        <v-card class="pa-5">
            <div class="d-flex align-center font-weight-medium mb-4">
                Configure custom cloud pricing
                <a href="https://docs.coroot.com/costs/" target="_blank" class="ml-2">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
                <v-progress-circular v-if="loading" indeterminate color="green" size="24" class="ml-2" />
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <p>If not overridden, Coroot uses GCP pricing for C4 machine family instances in the <i>us-central1</i> region</p>

            <v-form v-if="form" v-model="valid" ref="form">
                <div class="subtitle-1 mt-3">vCPU ($ per vCPU per hour)</div>
                <v-text-field
                    outlined
                    dense
                    v-model.number="form.per_cpu_core"
                    :rules="[$validators.isFloat, (v) => v > 0 || 'must be > 0']"
                    class="input"
                />

                <div class="subtitle-1 mt-3">Memory ($ per GB per hour)</div>
                <v-text-field
                    outlined
                    dense
                    v-model.number="form.per_memory_gb"
                    :rules="[$validators.isFloat, (v) => v > 0 || 'must be > 0']"
                    class="input"
                />

                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-3">
                    {{ error }}
                </v-alert>
                <v-alert v-if="message" color="green" outlined text class="mt-3">
                    {{ message }}
                </v-alert>
                <div class="mt-3 d-flex" style="gap: 8px">
                    <v-btn color="primary" @click="save" :disabled="!valid || !changed" :loading="loading">Save</v-btn>
                    <v-btn color="error" @click="reset" :disabled="!overridden" :loading="loading">Reset to defaults</v-btn>
                </div>
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script>
export default {
    components: {},

    data() {
        return {
            form: { per_cpu_core: 0, per_memory_gb: 0 },
            dialog: false,
            valid: false,
            loading: false,
            error: '',
            message: '',
            saved: {},
            overridden: false,
        };
    },

    mounted() {
        this.get();
    },
    computed: {
        changed() {
            return JSON.stringify(this.form) !== JSON.stringify(this.saved);
        },
    },
    watch: {
        dialog(v) {
            v && this.get();
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getCustomCloudPricing((data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form.per_cpu_core = data.per_cpu_core || 0;
                this.form.per_memory_gb = data.per_memory_gb || 0;
                this.overridden = !data.default;
                this.saved = JSON.parse(JSON.stringify(this.form));
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = JSON.parse(JSON.stringify(this.form));
            this.$api.saveCustomCloudPricing(form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                }, 3000);
                this.get();
            });
        },
        reset() {
            this.loading = true;
            this.error = '';
            this.message = '';

            this.$api.deleteCustomCloudPricing((error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully reset to defaults.';
                setTimeout(() => {
                    this.message = '';
                }, 3000);
                this.get();
            });
        },
    },
};
</script>

<style scoped></style>
