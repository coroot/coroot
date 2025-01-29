<template>
    <v-form v-model="valid" ref="form" style="max-width: 800px">
        <v-alert v-if="form.global" color="primary" outlined text>
            This project uses a global Prometheus configuration that can't be changed through the UI
        </v-alert>

        <div class="subtitle-1">Prometheus URL</div>
        <div class="caption">Coroot works on top of the telemetry data stored in your Prometheus server.</div>
        <v-text-field
            outlined
            dense
            v-model="form.url"
            :rules="[$validators.notEmpty, $validators.isUrl]"
            placeholder="https://prom.example.com:9090"
            hide-details="auto"
            class="flex-grow-1"
            single-line
            :disabled="form.global"
        />
        <v-checkbox
            v-model="form.tls_skip_verify"
            :disabled="!form.url.startsWith('https') || form.global"
            label="Skip TLS verify"
            hide-details
            class="my-2"
        />

        <v-checkbox v-model="basic_auth" label="HTTP basic auth" class="my-2" hide-details :disabled="form.global" />
        <div v-if="basic_auth" class="d-flex gap">
            <v-text-field outlined dense v-model="form.basic_auth.user" label="username" hide-details single-line :disabled="form.global" />
            <v-text-field
                v-model="form.basic_auth.password"
                label="password"
                type="password"
                outlined
                dense
                hide-details
                single-line
                :disabled="form.global"
            />
        </div>

        <v-checkbox v-model="custom_headers" label="Custom HTTP headers" class="my-2" hide-details :disabled="form.global" />
        <template v-if="custom_headers">
            <div v-for="(h, i) in form.custom_headers" :key="i" class="d-flex gap mb-2 align-center">
                <v-text-field outlined dense v-model="h.key" label="header" hide-details single-line :disabled="form.global" />
                <v-text-field outlined dense v-model="h.value" type="password" label="value" hide-details single-line :disabled="form.global" />
                <v-btn @click="form.custom_headers.splice(i, 1)" icon small :disabled="form.global">
                    <v-icon small>mdi-trash-can-outline</v-icon>
                </v-btn>
            </div>
            <v-btn color="primary" @click="form.custom_headers.push({ key: '', value: '' })" :disabled="form.global">Add header</v-btn>
        </template>

        <div class="subtitle-1 mt-3">Refresh interval</div>
        <div class="caption">
            How often Coroot retrieves telemetry data from a Prometheus. The value must be greater than the
            <a href="https://prometheus.io/docs/prometheus/latest/configuration/configuration/" target="_blank" rel="noopener noreferrer"
                ><var>scrape_interval</var></a
            >
            of the Prometheus server.
        </div>
        <v-select v-model="form.refresh_interval" :items="refreshIntervals" outlined dense :menu-props="{ offsetY: true }" :disabled="form.global" />

        <div class="subtitle-1">Extra selector</div>
        <div class="caption">An additional metric selector that will be added to every Prometheus query (e.g. <var>{cluster="us-west-1"}</var>)</div>
        <v-text-field outlined dense v-model="form.extra_selector" :rules="[$validators.isPrometheusSelector]" single-line :disabled="form.global" />

        <div class="subtitle-1">Remote Write URL</div>
        <div class="caption">
            If you're using a drop-in Prometheus replacement like VictoriaMetrics in cluster mode, you may need to configure a different Remote Write
            URL. By default, Coroot appends <var>/api/v1/write</var> to the base URL configured above.
        </div>
        <v-text-field outlined dense v-model="form.remote_write_url" :rules="[$validators.isUrl]" single-line :disabled="form.global" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{ message }}
        </v-alert>
        <v-btn block color="primary" @click="save" :disabled="!valid || form.global" :loading="loading">Save</v-btn>
    </v-form>
</template>

<script>
const refreshIntervals = [
    { value: 5000, text: '5 seconds' },
    { value: 10000, text: '10 seconds' },
    { value: 15000, text: '15 seconds' },
    { value: 30000, text: '30 seconds' },
    { value: 60000, text: '60 seconds' },
];

export default {
    data() {
        return {
            form: {
                url: '',
                tls_skip_verify: false,
                basic_auth: null,
                custom_headers: [],
                refresh_interval: 0,
                extra_selector: '',
            },
            basic_auth: false,
            custom_headers: true,
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
        custom_headers(v) {
            if (v && !this.form.custom_headers.length) {
                this.form.custom_headers.push({ key: '', value: '' });
            }
        },
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
            this.$api.getIntegrations('prometheus', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form = Object.assign({}, this.form, data);
                if (!this.form.basic_auth) {
                    this.form.basic_auth = { user: '', password: '' };
                    this.basic_auth = false;
                } else {
                    this.basic_auth = true;
                }
                if (!this.form.custom_headers) {
                    this.form.custom_headers = [];
                }
                this.custom_headers = !!this.form.custom_headers.length;
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            const form = JSON.parse(JSON.stringify(this.form));
            if (!this.basic_auth) {
                form.basic_auth = null;
            }
            if (!this.custom_headers) {
                form.custom_headers = [];
            }
            this.message = '';
            this.$api.saveIntegrations('prometheus', 'save', form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                this.message = 'Settings were successfully updated. The changes will take effect in a minute or two.';
                setTimeout(() => {
                    this.message = '';
                }, 3000);
                this.get();
            });
        },
    },
};
</script>

<style scoped>
.gap {
    gap: 16px;
}
</style>
