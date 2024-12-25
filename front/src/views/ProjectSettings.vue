<template>
    <v-form v-if="form" v-model="valid" ref="form" style="max-width: 800px">
        <v-alert v-if="readonly" color="primary" outlined text>
            This project is defined through the config and cannot be modified via the UI.
        </v-alert>
        <div class="caption">
            Project is a separate infrastructure or environment, e.g. <var>production</var>, <var>staging</var> or <var>prod-us-west</var>.
        </div>
        <v-text-field v-model="form.name" :rules="[$validators.isSlug]" :disabled="readonly" outlined dense required />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{ message }}
        </v-alert>
        <v-btn block color="primary" @click="save" :disabled="readonly || !valid" :loading="loading">Save</v-btn>
    </v-form>
</template>

<script>
export default {
    props: {
        projectId: String,
    },

    data() {
        return {
            form: {
                name: '',
            },
            readonly: false,
            valid: false,
            loading: false,
            error: '',
            message: '',
        };
    },

    mounted() {
        this.get();
    },

    watch: {
        projectId() {
            this.get();
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getProject(this.projectId, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.readonly = data.readonly;
                this.form.name = data.name;
                if (!this.projectId && this.$refs.form) {
                    this.$refs.form.resetValidation();
                }
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            this.$api.saveProject(this.projectId, this.form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('projects');
                this.message = 'Settings were successfully updated.';
                if (!this.projectId) {
                    const projectId = data.trim();
                    this.$router.replace({ name: 'project_settings', params: { projectId, tab: 'prometheus' } }).catch((err) => err);
                }
            });
        },
    },
};
</script>

<style scoped></style>
