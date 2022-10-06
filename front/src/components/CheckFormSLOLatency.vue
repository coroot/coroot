<template>
    <v-form v-if="form" v-model="valid">
        <div v-for="c in form.configs">
            Histogram query:
            <MetricSelector v-model="c.histogram_query" :rules="[$validators.notEmpty]" wrap="sum by(le)( rate( <input> [..]) )" class="mb-3"/>

            Objective bucket:
            <v-text-field outlined dense v-model="c.objective_bucket" :rules="[$validators.notEmpty]" hide-details class="input" />

            Objective percentage:
            <v-text-field outlined dense v-model.number="c.objective_percentage" :rules="[$validators.isFloat]" hide-details class="input" />
        </div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{error}}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{message}}
        </v-alert>
        <v-btn block color="primary" @click="save" :disabled="!(valid && changed)" :loading="saving" class="mt-5">
            Save
        </v-btn>
    </v-form>
</template>

<script>
import MetricSelector from "@/components/MetricSelector";

export default {
    components: {MetricSelector},
    props: {
        appId: String,
        check: Object,
        open: Boolean,
    },

    data() {
        return {
            loading: false,
            error: '',
            message: '',
            form: null,
            saved: '',
            valid: false,
            saving: false,
        }
    },

    mounted() {
        this.get();
    },

    watch: {
        open() {
            this.open && this.get();
        }
    },

    computed: {
        changed() {
            return !!this.form && this.saved !== JSON.stringify(this.form);
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.$api.getCheckConfig(this.appId, this.check.id, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    this.form = null;
                    return;
                }
                this.form = data;
                this.saved = JSON.stringify(this.form);
            })
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveCheckConfig(this.appId, this.check.id, this.form, (data, error) => {
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
            })
        },
    }
}
</script>

<style scoped>
.input >>> .v-input__slot {
    min-height: initial !important;
    padding: 0 8px !important;
}
</style>