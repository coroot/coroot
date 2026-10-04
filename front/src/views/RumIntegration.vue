<template>
    <v-dialog v-model="dialog" max-width="800">
        <template #activator="{ on, attrs }">
            <v-btn :color="color" :outlined="outlined" :small="small" v-bind="attrs" v-on="on">
                <slot></slot>
            </v-btn>
        </template>
        <v-card class="pa-5">
            <div class="d-flex align-center text-h5 mb-4">
                RUM Integration
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <p>
                Coroot Real User Monitoring collects Core Web Vitals, page loads, fetch/XHR, and JS errors from browsers via OpenTelemetry
                (OTLP/HTTP). Data is stored in dedicated <code>rum_*</code> ClickHouse tables and correlated with backend traces through
                <code>traceparent</code>.
            </p>

            <v-form v-model="valid">
                <div class="subtitle-2 mt-2">Coroot URL (must be reachable from browsers):</div>
                <v-text-field
                    v-model="coroot_url"
                    :rules="[$validators.notEmpty, $validators.isUrl]"
                    placeholder="https://coroot.example.com"
                    outlined
                    dense
                    hide-details
                />

                <div class="subtitle-2 mt-2">RUM API Key (type = rum, with allowed origins):</div>
                <v-select
                    v-model="api_key"
                    :rules="[$validators.notEmpty]"
                    :items="rumKeys"
                    outlined
                    dense
                    hide-details
                    :menu-props="{ offsetY: true }"
                    no-data-text="Create a RUM API key in project settings"
                />

                <div class="subtitle-2 mt-2">Service name:</div>
                <v-text-field v-model="service_name" :rules="[$validators.notEmpty, $validators.isSlug]" placeholder="web-shop" outlined dense />
            </v-form>

            <div class="subtitle-2 mt-4">Snippet</div>
            <Code :disabled="!valid">
                <pre>
&lt;script src="{{ coroot_url }}/static/rum/coroot-rum.js"&gt;&lt;/script&gt;
&lt;script&gt;
  CorootRum.init({
    endpoint: "{{ coroot_url }}",
    apiKey: "{{ api_key }}",
    serviceName: "{{ service_name }}",
    sampleRate: 0.1,
    allowedTraceUrls: [/https:\/\/api\.example\.com/],
  });
&lt;/script&gt;
                </pre>
            </Code>

            <p class="mt-4">
                Create a RUM key with allowed origins in
                <router-link :to="{ name: 'project_settings' }"><span @click="dialog = false">project settings</span></router-link
                >. Propagate <code>traceparent</code> only to your API origins so browser spans join backend traces on the Service Map.
            </p>
        </v-card>
    </v-dialog>
</template>

<script>
import Code from '@/components/Code.vue';

export default {
    components: { Code },
    props: {
        color: { type: String, default: 'primary' },
        outlined: Boolean,
        small: Boolean,
    },
    data() {
        return {
            dialog: false,
            valid: false,
            coroot_url: window.location.origin,
            api_key: '',
            service_name: 'web-app',
            api_keys: [],
        };
    },
    computed: {
        rumKeys() {
            if (!Array.isArray(this.api_keys)) return [];
            return this.api_keys.filter((k) => k.type === 'rum').map((k) => ({ value: k.key, text: `${k.key} (${k.description})` }));
        },
    },
    watch: {
        dialog(v) {
            if (v) this.load();
        },
    },
    methods: {
        load() {
            const projectId = this.$route.params.projectId || '';
            this.$api.getProject(projectId, (data, error) => {
                if (error) {
                    this.api_keys = [];
                    return;
                }
                this.api_keys = (data && data.api_keys) || [];
                if (this.api_keys === 'permission denied') this.api_keys = [];
                if (!this.api_key && this.rumKeys.length === 1) {
                    this.api_key = this.rumKeys[0].value;
                }
            });
        },
    },
};
</script>
