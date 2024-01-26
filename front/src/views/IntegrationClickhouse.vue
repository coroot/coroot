<template>
    <v-form v-if="form" v-model="valid" ref="form" style="max-width: 800px">
        <div class="subtitle-1">Protocol</div>
        <v-radio-group v-model="form.protocol" row dense class="mt-0">
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
        />

        <div class="subtitle-1 mt-3">Credentials</div>
        <div class="d-flex gap">
            <v-text-field v-model="form.auth.user" :rules="[$validators.notEmpty]" label="username" outlined dense hide-details single-line />
            <v-text-field
                v-model="form.auth.password"
                :rules="[$validators.notEmpty]"
                label="password"
                type="password"
                outlined
                dense
                hide-details
                single-line
            />
        </div>

        <div class="subtitle-1 mt-3">Database</div>
        <v-text-field v-model="form.database" :rules="[$validators.notEmpty]" outlined dense hide-details single-line />

        <div class="d-flex" style="gap: 8px">
            <div>
                <div class="d-flex align-center">
                    <v-checkbox v-model="traces" label="Use Clickhouse as a datasource for traces" hide-details class="mt-3" />
                    <a href="https://coroot.com/docs/coroot-community-edition/tracing" target="_blank" class="mt-3 ml-1">
                        <v-icon>mdi-information-outline</v-icon>
                    </a>
                </div>
                <div class="d-flex align-center">
                    <v-checkbox v-model="logs" label="Use Clickhouse as a datasource for logs" hide-details class="mt-5" />
                    <a href="https://coroot.com/docs/coroot-community-edition/logs" target="_blank" class="mt-5 ml-1">
                        <v-icon>mdi-information-outline</v-icon>
                    </a>
                </div>
                <div class="d-flex align-center">
                    <v-checkbox v-model="profiles" label="Use Clickhouse as a datasource for profiles" hide-details class="mt-5" />
                    <a href="https://coroot.com/docs/coroot-community-edition/profiling" target="_blank" class="mt-5 ml-1">
                        <v-icon>mdi-information-outline</v-icon>
                    </a>
                </div>
            </div>
            <div class="flex-grow-1">
                <v-text-field
                    v-model="form.traces_table"
                    :disabled="!traces"
                    label="traces table name"
                    prepend-inner-icon="mdi-table"
                    outlined
                    dense
                    hide-details
                    single-line
                    class="mt-2"
                />
                <v-text-field
                    v-model="form.logs_table"
                    :disabled="!logs"
                    label="logs table name"
                    prepend-inner-icon="mdi-table"
                    outlined
                    dense
                    hide-details
                    single-line
                    class="mt-2"
                />
            </div>
        </div>

        <v-checkbox v-model="form.tls_enable" label="Enable TLS" hide-details class="my-3" />
        <v-checkbox v-model="form.tls_skip_verify" :disabled="!form.tls_enable" label="Skip TLS verify" hide-details class="my-2" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{ message }}
        </v-alert>
        <div class="mt-3">
            <v-btn v-if="saved.addr && !form.addr" block color="error" @click="del" :loading="loading">Delete</v-btn>
            <v-btn v-else block color="primary" @click="save" :disabled="!form.addr || !valid" :loading="loading">Test & Save</v-btn>
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

            traces: true,
            logs: true,
            profiles: true,
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

    watch: {
        traces(v) {
            this.form.traces_table = v ? 'otel_traces' : '';
        },
        logs(v) {
            this.form.logs_table = v ? 'otel_logs' : '';
        },
        profiles(v) {
            this.form.profiling_disabled = !v;
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
                this.traces = !!this.form.traces_table;
                this.logs = !!this.form.logs_table;
                this.profiles = !this.form.profiling_disabled;
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = JSON.parse(JSON.stringify(this.form));
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
    },
};
</script>

<style scoped>
.gap {
    gap: 16px;
}
</style>
