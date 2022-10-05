<template>
    <v-dialog :value="value" max-width="800">
        <v-card v-if="loading" class="pa-10">
            <v-progress-linear indeterminate />
        </v-card>
        <v-card v-else class="pa-4">
            <div class="d-flex align-center font-weight-medium mb-4">
                Configure "{{ check.title }}" check
                <v-spacer />
                <v-btn icon @click="$emit('input', false)"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <v-form v-model="valid">
                <div v-for="c in configs">
                    Total requests query:
                    <v-text-field outlined v-model="c.total_requests_query" />
                    Failed requests query:
                    <v-text-field outlined v-model="c.failed_requests_query" />
                    Objective percentage:
                    <v-text-field outlined v-model.number="c.objective_percentage" :rules="[$validators.isFloat]" />
                </div>
            </v-form>
            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                {{error}}
            </v-alert>
            <v-alert v-if="message && !changed" color="green" outlined text>
                {{message}}
            </v-alert>
            <v-btn block color="primary" @click="save" :disabled="!(valid && changed)" :loading="saving" class="mt-5">
                Save
            </v-btn>
        </v-card>
    </v-dialog>
</template>

<script>
export default {
    props: {
        appId: String,
        check: Object,
        value: Boolean,
    },

    data() {
        return {
            loading: false,
            error: '',
            message: '',
            configs: null,
            saved: '',
            valid: false,
            saving: false,
        }
    },

    watch: {
        value() {
            this.value && this.get();
        }
    },

    computed: {
        changed() {
            return !!this.configs.length && this.saved !== JSON.stringify(this.toForm());
        },
    },

    methods: {
        toForm() {
            return {configs: this.configs};
        },
        get() {
            this.loading = true;
            this.$api.getCheckConfig(this.appId, this.check.id, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    this.configs = null;
                    return;
                }
                this.configs = data;
                this.saved = JSON.stringify(this.toForm());
            })
        },
        save() {
            const form = this.toForm();
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveCheckConfig(this.appId, this.check.id, form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                this.message = 'Settings were successfully updated.';
                this.get();
            })
        },
    }
}
</script>

<style scoped>
</style>