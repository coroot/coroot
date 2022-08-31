<template>
<div class="mx-auto" style="max-width: 800px">
    <h1 class="text-h5 my-5">
        Project settings
        <v-progress-linear v-if="loading" indeterminate color="green" />
    </h1>

    <v-form v-model="valid">
        <div class="subtitle-1">Project name</div>
        <div class="caption">
            Project is a separate infrastructure or environment with a dedicated Prometheus, e.g. <var>production</var>, <var>staging</var> or <var>prod-us-west</var>.
        </div>
        <v-text-field v-model="form.name" :rules="[$validators.isSlug]" outlined dense required/>

        <div class="subtitle-1">Prometheus URL</div>
        <div class="caption">
            Coroot works on top of the telemetry data stored in your Prometheus server.
        </div>
        <v-text-field outlined dense v-model="form.prometheus.url" :rules="[$validators.isUrl]" placeholder="https://prom.example.com:9090" class="flex-grow-1" />
        <v-checkbox v-model="form.prometheus.tls_skip_verify" :disabled="!form.prometheus.url.startsWith('https')" label="Skip TLS verify" class="mt-1" />
        <div class="d-md-flex gap">
            <v-checkbox v-model="form.prometheus.basic_auth" :true-value="{user: '', password: ''}" :false-value="null" label="HTTP basic auth" class="mt-1" />
            <template v-if="!!form.prometheus.basic_auth">
                <v-text-field outlined dense v-model="form.prometheus.basic_auth.user" label="username"  />
                <v-text-field v-model="form.prometheus.basic_auth.password" label="password" type="password" outlined dense />
            </template>
        </div>

        <div class="subtitle-1">Refresh interval</div>
        <div class="caption">
            How often Coroot retrieves telemetry data from a Prometheus.
        </div>
        <v-select v-model="form.prometheus.refresh_interval" :items="refreshIntervals" outlined dense :menu-props="{offsetY: true}" />
    </v-form>
    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
        {{error}}
    </v-alert>
    <v-btn block color="primary" @click="post" :disabled="!valid" class="mt-5">Save</v-btn>
</div>
</template>

<script>
const refreshIntervals = [
    {value: 30 * 1000, text: '30 seconds'},
    {value: 60 * 1000, text: '1 minute'},
    {value: 2 * 60 * 1000, text: '2 minutes'},
    {value: 5 * 60 * 1000, text: '5 minutes'},
];

export default {
    props: {
        projectId: String,
    },

    data() {
        return {
            form: {
                name: '',
                prometheus: {
                    url: '',
                    tls_skip_verify: false,
                    basic_auth: null,
                    refresh_interval: refreshIntervals[0].value,
                },
            },
            valid: false,
            loading: false,
            error: '',
        }
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
            if (!this.projectId) {
                return;
            }
            this.loading = true;
            this.error = '';
            this.$api.getProject(this.projectId, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form = data;
            })
        },
        post() {
            this.loading = true;
            this.error = '';
            this.$api.saveProject(this.projectId, this.form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                const projectId = data.trim();
                this.$router.replace({name: 'project_settings', params: {projectId}}).catch(err => err);
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
