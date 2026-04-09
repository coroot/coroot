<template>
    <div>
        <!-- eslint-disable vue/no-mutating-props -->
        <div class="subtitle-1">Webhook URL</div>
        <v-text-field v-model="form.url" outlined dense :rules="[$validators.notEmpty]" hide-details />

        <v-checkbox v-model="form.tls_skip_verify" :disabled="!(form.url || '').startsWith('https')" label="Skip TLS verify" hide-details />

        <v-checkbox v-model="basic_auth" label="HTTP basic auth" class="my-2" hide-details />
        <div v-if="basic_auth" class="d-flex gap">
            <v-text-field outlined dense v-model="form.basic_auth.user" label="username" hide-details single-line />
            <v-text-field v-model="form.basic_auth.password" label="password" type="password" outlined dense hide-details single-line />
        </div>

        <v-checkbox v-model="custom_headers" label="Custom HTTP headers" class="my-2" hide-details />
        <template v-if="custom_headers">
            <div v-for="(h, i) in form.custom_headers" :key="i" class="d-flex mb-2 align-center" style="gap: 8px">
                <v-text-field outlined dense v-model="h.key" label="header" hide-details single-line />
                <v-text-field outlined dense v-model="h.value" label="value" hide-details single-line />
                <v-btn @click="form.custom_headers.splice(i, 1)" icon small>
                    <v-icon small>mdi-trash-can-outline</v-icon>
                </v-btn>
            </div>
            <v-btn color="primary" small @click="form.custom_headers.push({ key: '', value: '' })">Add header</v-btn>
        </template>

        <div class="subtitle-1 mt-5">Custom fields</div>
        <div class="caption mb-2">Static key-value pairs included in template data</div>
        <div v-for="(entry, i) in customFieldsList" :key="i" class="d-flex mb-2 align-center" style="gap: 8px">
            <v-text-field outlined dense v-model="entry.key" label="key" hide-details single-line @input="syncCustomFields" />
            <v-text-field outlined dense v-model="entry.value" label="value" hide-details single-line @input="syncCustomFields" />
            <v-btn
                @click="
                    customFieldsList.splice(i, 1);
                    syncCustomFields();
                "
                icon
                small
            >
                <v-icon small>mdi-trash-can-outline</v-icon>
            </v-btn>
        </div>
        <v-btn color="primary" small @click="customFieldsList.push({ key: '', value: '' })">Add field</v-btn>

        <div class="subtitle-1 mt-5">Notify of</div>
        <v-checkbox v-model="form.incidents" label="Incidents" dense hide-details />
        <v-checkbox v-model="form.deployments" label="Deployments" dense hide-details />
        <v-checkbox v-model="form.alerts" label="Alerts" dense />

        <div class="subtitle-1">Incident template</div>
        <v-textarea v-model="form.incident_template" outlined dense :rules="form.incidents ? [$validators.notEmpty] : []" />

        <div class="subtitle-1">Deployment template</div>
        <v-textarea v-model="form.deployment_template" outlined dense :rules="form.deployments ? [$validators.notEmpty] : []" />

        <div class="subtitle-1">Alert template</div>
        <v-textarea v-model="form.alert_template" outlined dense :rules="form.alerts ? [$validators.notEmpty] : []" />
        <!-- eslint-enable vue/no-mutating-props -->
    </div>
</template>

<script>
export default {
    props: {
        form: Object,
    },

    data() {
        const customFieldsList = [];
        if (this.form.custom_fields) {
            for (const [key, value] of Object.entries(this.form.custom_fields)) {
                customFieldsList.push({ key, value });
            }
        }
        return {
            basic_auth: !!this.form.basic_auth,
            custom_headers: !!this.form.custom_headers,
            customFieldsList,
        };
    },

    methods: {
        syncCustomFields() {
            const fields = {};
            for (const entry of this.customFieldsList) {
                if (entry.key) {
                    fields[entry.key] = entry.value;
                }
            }
            // eslint-disable-next-line vue/no-mutating-props
            this.form.custom_fields = fields;
        },
    },

    watch: {
        form() {
            if (!this.form.basic_auth) {
                // eslint-disable-next-line vue/no-mutating-props
                this.form.basic_auth = { user: '', password: '' };
            }
            if (!this.form.custom_headers) {
                // eslint-disable-next-line vue/no-mutating-props
                this.form.custom_headers = [];
            }
            this.customFieldsList = [];
            if (this.form.custom_fields) {
                for (const [key, value] of Object.entries(this.form.custom_fields)) {
                    this.customFieldsList.push({ key, value });
                }
            }
        },
    },
};
</script>

<style scoped></style>
