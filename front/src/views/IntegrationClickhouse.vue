<template>
    <v-form v-if="form" v-model="valid" ref="form" style="max-width: 800px">
        <v-alert v-if="form.global" color="primary" outlined text>
            This project uses a global ClickHouse configuration that can't be changed through the UI
        </v-alert>

        <div class="subtitle-1">Protocol</div>
        <v-radio-group v-model="form.protocol" row dense class="mt-0" :disabled="form.global">
            <v-radio label="Native" value="native"></v-radio>
            <v-radio label="HTTP" value="http"></v-radio>
        </v-radio-group>

        <div class="subtitle-1">Clickhouse address</div>
        <div class="caption"></div>
        <v-text-field
            outlined
            dense
            v-model="form.addr"
            :rules="[$validators.isAddr]"
            placeholder="clickhouse:9000"
            hide-details="auto"
            class="flex-grow-1"
            clearable
            single-line
            :disabled="form.global"
        />

        <div class="subtitle-1 mt-3">Credentials</div>
        <div class="d-flex gap">
            <v-text-field
                v-model="form.auth.user"
                :rules="[$validators.notEmpty]"
                label="username"
                outlined
                dense
                hide-details
                single-line
                :disabled="form.global"
            />
            <v-text-field
                v-model="form.auth.password"
                label="password"
                type="password"
                outlined
                dense
                hide-details
                single-line
                :disabled="form.global"
            />
        </div>

        <div class="subtitle-1 mt-3">Database</div>
        <v-text-field v-model="form.database" :rules="[$validators.notEmpty]" outlined dense hide-details single-line :disabled="form.global" />

        <v-checkbox v-model="form.tls_enable" label="Enable TLS" hide-details class="my-3" :disabled="form.global" />
        <v-checkbox v-model="form.tls_skip_verify" :disabled="!form.tls_enable || form.global" label="Skip TLS verify" hide-details class="my-2" />

        <div class="subtitle-1 mt-3">Logs TTL</div>
        <v-text-field
            v-model="logsTTLInput"
            outlined
            dense
            hide-details
            single-line
            placeholder="e.g., 1h, 30m, 2d, 1w"
            :disabled="form.global || (saved.database === form.database && saved.addr === form.addr)"
        />

        <div class="subtitle-1 mt-3">Traces TTL</div>
        <v-text-field
            v-model="tracesTTLInput"
            outlined
            dense
            hide-details
            single-line
            placeholder="e.g., 1h, 30m, 2d, 1w"
            :disabled="form.global || (saved.database === form.database && saved.addr === form.addr)"
        />

        <div class="subtitle-1 mt-3">Profiles TTL</div>
        <v-text-field
            v-model="profilesTTLInput"
            outlined
            dense
            hide-details
            single-line
            placeholder="e.g., 1h, 30m, 2d, 1w"
            :disabled="form.global || (saved.database === form.database && saved.addr === form.addr)"
        />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{ message }}
        </v-alert>
        <div class="mt-3">
            <v-btn v-if="saved.addr && !form.addr" block color="error" @click="del" :loading="loading">Delete</v-btn>
            <v-btn v-else block color="primary" @click="save" :disabled="!form.addr || !valid || form.global" :loading="loading">Test & Save</v-btn>
        </div>
    </v-form>
</template>

<script>
export default {
    data() {
        return {
            form: null,
            valid: false,
            loading: false,
            error: '',
            message: '',
            saved: null,
            logsTTLInput: '', // User-friendly input (e.g., "1h")
            tracesTTLInput: '',
            profilesTTLInput: '',
            DEFAULT_TTL: '1w', // Default to 7 days
            DEFAULT_TTL_NS: 604800000000000, // 7 days in nanoseconds
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

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getIntegrations('clickhouse', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form = data;
                this.saved = JSON.parse(JSON.stringify(this.form));
                this.logsTTLInput = this.formatDuration(this.form.logs_ttl) || this.DEFAULT_TTL;
                this.tracesTTLInput = this.formatDuration(this.form.traces_ttl) || this.DEFAULT_TTL;
                this.profilesTTLInput = this.formatDuration(this.form.profiles_ttl) || this.DEFAULT_TTL;
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = JSON.parse(JSON.stringify(this.form));
            if (this.saved.addr === form.addr && this.saved.database === form.database) {
                form.logs_ttl = this.saved.logs_ttl;
                form.traces_ttl = this.saved.traces_ttl;
                form.profiles_ttl = this.saved.profiles_ttl;
            } else {
                form.logs_ttl = this.parseDuration(this.logsTTLInput);
                form.traces_ttl = this.parseDuration(this.tracesTTLInput);
                form.profiles_ttl = this.parseDuration(this.profilesTTLInput);
            }

            this.$api.saveIntegrations('clickhouse', 'save', form, (data, error) => {
                this.loading = false;
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
            });
        },
        del() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations('clickhouse', 'del', null, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.get();
            });
        },

        parseDuration(input) {
            if (!input) return this.DEFAULT_TTL_NS;
            const units = {
                s: 1e9, // seconds to nanoseconds
                m: 60e9, // minutes to nanoseconds
                h: 3600e9, // hours to nanoseconds
                d: 86400e9, // days to nanoseconds
                w: 604800e9, // weeks to nanoseconds (7 days)
            };

            const match = input.match(/^(\d+)([smhdw])$/);
            if (!match) return this.DEFAULT_TTL_NS;

            const value = parseInt(match[1], 10);
            const unit = match[2];

            let ttl = value * units[unit] || this.DEFAULT_TTL_NS;

            // If TTL is ≤ 1 minute, default to 1w
            if (ttl <= 60e9) {
                return this.DEFAULT_TTL_NS;
            }

            return ttl;
        },

        formatDuration(ns) {
            if (!ns) return this.DEFAULT_TTL;
            const seconds = ns / 1e9;
            if (seconds % 604800 === 0) return `${seconds / 604800}w`;
            if (seconds % 86400 === 0) return `${seconds / 86400}d`;
            if (seconds % 3600 === 0) return `${seconds / 3600}h`;
            if (seconds % 60 === 0) return `${seconds / 60}m`;
            return `${seconds}s`;
        },
    },
};
</script>

<style scoped>
.gap {
    gap: 16px;
}
</style>
