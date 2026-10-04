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

                <div class="subtitle-2 mt-2">RUM API Key:</div>
                <v-select
                    v-model="api_key"
                    :rules="[$validators.notEmpty]"
                    :items="keyItems"
                    outlined
                    dense
                    hide-details
                    :menu-props="{ offsetY: true }"
                    :disabled="!editable && !rumKeys.length"
                    no-data-text="No RUM keys — create one below or in project settings"
                />

                <template v-if="creatingNew">
                    <div class="subtitle-2 mt-2">Key description:</div>
                    <v-text-field
                        v-model="newDescription"
                        :rules="[$validators.notEmpty]"
                        placeholder="web-shop RUM"
                        outlined
                        dense
                        hide-details
                        :disabled="!editable"
                    />
                </template>

                <div class="subtitle-2 mt-2">Allowed domains (one per line):</div>
                <v-textarea
                    v-model="originsText"
                    outlined
                    dense
                    rows="3"
                    placeholder="shop.example.com&#10;example.com/shop&#10;example.com/portal&#10;localhost:3000"
                    hint="Hostname, optional path prefix (example.com/shop), *.example.com, or *. Required for the RUM key."
                    persistent-hint
                    :readonly="!editable"
                    :disabled="!creatingNew && !api_key"
                />
                <div v-if="editable" class="d-flex align-center mt-1 mb-2">
                    <v-spacer />
                    <v-btn
                        v-if="creatingNew"
                        small
                        color="primary"
                        :disabled="!newDescription.trim() || !originsText.trim()"
                        :loading="saving"
                        @click="generateKey"
                    >
                        Generate RUM key
                    </v-btn>
                    <v-btn
                        v-else-if="api_key"
                        small
                        color="primary"
                        outlined
                        :disabled="!originsText.trim() || !domainsDirty"
                        :loading="saving"
                        @click="saveDomains"
                    >
                        Save domains
                    </v-btn>
                </div>
                <v-alert v-if="error" class="mt-2" color="red" icon="mdi-alert-octagon-outline" outlined text dense>
                    {{ error }}
                </v-alert>

                <div class="subtitle-2 mt-2">Service name:</div>
                <v-text-field v-model="service_name" :rules="[$validators.notEmpty, $validators.isSlug]" placeholder="web-shop" outlined dense />
            </v-form>

            <div class="subtitle-2 mt-4">Snippet</div>
            <Code :disabled="!snippetReady">
                <pre>
&lt;script src="{{ coroot_url }}/static/rum/coroot-rum.js"&gt;&lt;/script&gt;
&lt;script&gt;
  CorootRum.init({
    endpoint: "{{ coroot_url }}",
    apiKey: "{{ snippetApiKey }}",
    serviceName: "{{ service_name }}",
    sampleRate: 0.1,
    allowedTraceUrls: [/https:\/\/api\.example\.com/],
  });
&lt;/script&gt;
                </pre>
            </Code>

            <p class="mt-4">
                Telemetry is accepted only from the allowed domains (and path prefixes) on the RUM key. Propagate
                <code>traceparent</code> only to your API origins so browser spans join backend traces on the Service Map.
            </p>
        </v-card>
    </v-dialog>
</template>

<script>
import Code from '@/components/Code.vue';

const NEW_KEY = '__new__';

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
            editable: false,
            saving: false,
            error: '',
            coroot_url: window.location.origin,
            api_key: '',
            service_name: 'web-app',
            api_keys: [],
            originsText: '',
            savedOriginsText: '',
            newDescription: '',
        };
    },
    computed: {
        rumKeys() {
            if (!Array.isArray(this.api_keys)) return [];
            return this.api_keys
                .filter((k) => k.type === 'rum' && k.key)
                .map((k) => ({
                    value: k.key,
                    text: `${k.key} (${k.description || 'rum'})`,
                    allowed_origins: k.allowed_origins || [],
                }));
        },
        keyItems() {
            const items = [...this.rumKeys];
            if (this.editable) {
                items.push({ value: NEW_KEY, text: '+ Create new RUM key' });
            }
            return items;
        },
        creatingNew() {
            return this.api_key === NEW_KEY;
        },
        domainsDirty() {
            return this.originsText.trim() !== this.savedOriginsText.trim();
        },
        snippetReady() {
            return this.valid && !!this.api_key && this.api_key !== NEW_KEY;
        },
        snippetApiKey() {
            if (!this.api_key || this.api_key === NEW_KEY) {
                return '<generate-key-first>';
            }
            return this.api_key;
        },
        selectedKey() {
            return this.api_keys.find((k) => k.key === this.api_key) || null;
        },
    },
    watch: {
        dialog(v) {
            if (v) this.load();
        },
        api_key(key) {
            this.error = '';
            if (key === NEW_KEY) {
                this.originsText = '';
                this.savedOriginsText = '';
                if (!this.newDescription) this.newDescription = this.service_name || 'web-app';
                return;
            }
            const k = this.api_keys.find((x) => x.key === key);
            const origins = (k && k.allowed_origins) || [];
            this.originsText = origins.join('\n');
            this.savedOriginsText = this.originsText;
        },
    },
    methods: {
        parseOrigins() {
            return this.originsText
                .split('\n')
                .map((s) => s.trim())
                .filter(Boolean);
        },
        load() {
            this.error = '';
            this.saving = false;
            this.$api.apiKeys(null, (data, error) => {
                if (error) {
                    this.api_keys = [];
                    this.editable = false;
                    this.error = error;
                    return;
                }
                this.editable = !!(data && data.editable);
                this.api_keys = (data && data.keys) || [];
                if (!this.api_key || (this.api_key !== NEW_KEY && !this.api_keys.some((k) => k.key === this.api_key))) {
                    if (this.rumKeys.length === 1) {
                        this.api_key = this.rumKeys[0].value;
                    } else if (this.editable && !this.rumKeys.length) {
                        this.api_key = NEW_KEY;
                    } else {
                        this.api_key = '';
                    }
                } else if (this.api_key && this.api_key !== NEW_KEY) {
                    const k = this.api_keys.find((x) => x.key === this.api_key);
                    const origins = (k && k.allowed_origins) || [];
                    this.originsText = origins.join('\n');
                    this.savedOriginsText = this.originsText;
                }
            });
        },
        generateKey() {
            this.error = '';
            const allowed_origins = this.parseOrigins();
            if (!allowed_origins.length) {
                this.error = 'allowed domains are required';
                return;
            }
            this.saving = true;
            this.$api.apiKeys(
                {
                    action: 'generate',
                    description: this.newDescription.trim(),
                    type: 'rum',
                    allowed_origins,
                },
                (data, error) => {
                    this.saving = false;
                    if (error) {
                        this.error = error;
                        return;
                    }
                    // Reload keys and select the newly created one (last rum key matching description).
                    this.$api.apiKeys(null, (keysData, keysErr) => {
                        if (keysErr) {
                            this.error = keysErr;
                            return;
                        }
                        this.editable = !!(keysData && keysData.editable);
                        this.api_keys = (keysData && keysData.keys) || [];
                        const created = [...this.api_keys]
                            .reverse()
                            .find((k) => k.type === 'rum' && k.description === this.newDescription.trim());
                        if (created) {
                            this.api_key = created.key;
                        }
                    });
                },
            );
        },
        saveDomains() {
            this.error = '';
            const allowed_origins = this.parseOrigins();
            if (!this.selectedKey || !allowed_origins.length) {
                this.error = 'allowed domains are required';
                return;
            }
            this.saving = true;
            this.$api.apiKeys(
                {
                    action: 'edit',
                    key: this.selectedKey.key,
                    description: this.selectedKey.description || '',
                    type: 'rum',
                    allowed_origins,
                },
                (data, error) => {
                    this.saving = false;
                    if (error) {
                        this.error = error;
                        return;
                    }
                    this.savedOriginsText = this.originsText;
                    const idx = this.api_keys.findIndex((k) => k.key === this.selectedKey.key);
                    if (idx >= 0) {
                        this.$set(this.api_keys, idx, { ...this.api_keys[idx], allowed_origins });
                    }
                },
            );
        },
    },
};
</script>
