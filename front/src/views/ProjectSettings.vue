<template>
    <v-form v-if="form" v-model="valid" ref="form">
        <div class="subtitle-1">Project name</div>
        <div class="caption">
            Project is a separate infrastructure or environment with a dedicated Prometheus, e.g. <var>production</var>, <var>staging</var> or <var>prod-us-west</var>.
        </div>
        <v-text-field v-model="form.name" :rules="[$validators.isSlug]" outlined dense required/>

        <div class="subtitle-1">Prometheus URL</div>
        <div class="caption">
            Coroot works on top of the telemetry data stored in your Prometheus server.
        </div>
        <v-text-field outlined dense v-model="form.prometheus.url" :rules="[$validators.isUrl]" placeholder="https://prom.example.com:9090" hide-details="auto" class="flex-grow-1" />
        <v-checkbox v-model="form.prometheus.tls_skip_verify" :disabled="!form.prometheus.url.startsWith('https')" label="Skip TLS verify" hide-details class="mt-1" />
        <div class="d-md-flex gap">
            <v-checkbox v-model="basic_auth" label="HTTP basic auth" class="mt-1" />
            <template v-if="basic_auth">
                <v-text-field outlined dense v-model="form.prometheus.basic_auth.user" label="username"  />
                <v-text-field v-model="form.prometheus.basic_auth.password" label="password" type="password" outlined dense />
            </template>
        </div>

        <div class="subtitle-1">Refresh interval</div>
        <div class="caption">
            How often Coroot retrieves telemetry data from a Prometheus.
            The value must be greater than the <a href="https://prometheus.io/docs/prometheus/latest/configuration/configuration/" target="_blank" rel="noopener noreferrer"><var>scrape_interval</var></a> of the Prometheus server.
        </div>
        <v-select v-model="form.prometheus.refresh_interval" :items="refreshIntervals" outlined dense :menu-props="{offsetY: true}" />
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{error}}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{message}}
        </v-alert>
        <v-btn block color="primary" @click="save" :disabled="!valid" :loading="loading">Save</v-btn>
    </v-form>
</template>

<script>
const refreshIntervals = [
    {value: 5000, text: '5 seconds'},
    {value: 10000, text: '10 seconds'},
    {value: 15000, text: '15 seconds'},
    {value: 30000, text: '30 seconds'},
    {value: 60000, text: '60 seconds'},
];

export default {
    props: {
        projectId: String,
    },

    data() {
        return {
            form: null,
            basic_auth: false,
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
        }
    },

    computed: {
        refreshIntervals() {
            return refreshIntervals;
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
                this.form = data;
                if (!this.form.prometheus.basic_auth) {
                    this.form.prometheus.basic_auth = {user: '', password: ''};
                    this.basic_auth = false;
                } else {
                    this.basic_auth = true;
                }
                if (!this.projectId && this.$refs.form) {
                    this.$refs.form.resetValidation();
                }
            })
        },
        save() {
            this.loading = true;
            this.error = '';
            const form = JSON.parse(JSON.stringify(this.form));
            if (!this.basic_auth) {
                form.prometheus.basic_auth = null;
            }
            this.message = '';
            this.$api.saveProject(this.projectId, form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('project-saved');
                this.message = 'Settings were successfully updated. The changes will take effect in a minute or two.';
                if (!this.projectId) {
                    const projectId = data.trim();
                    this.$router.replace({name: 'project_settings', params: {projectId}}).catch(err => err);
                }
            })
        },
    },
}
</script>

<style scoped>
.gap {
    gap: 16px;
}
</style>
