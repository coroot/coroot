<template>
    <v-dialog v-model="dialog" max-width="800">
        <template #activator="{ on }">
            <v-btn v-if="active" v-on="on" small icon style="position: absolute; top: 10px; right: 10px">
                <v-icon>mdi-cog</v-icon>
            </v-btn>
            <div v-else class="mb-2">
                It seems this app is a {{ types[type].name }} database.
                <v-btn v-on="on" color="primary" small>Configure</v-btn>
                <br />
                If you just configured the integration, please wait a couple minutes for it to collect data.
            </div>
        </template>
        <v-card class="pa-5">
            <div class="d-flex align-center font-weight-medium mb-4">
                Configure {{ types[type].name }} integration
                <v-progress-circular v-if="loading" indeterminate color="green" size="24" class="ml-2" />
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <template v-if="type === 'postgres'">
                <p>
                    This integration allows Coroot to collect Postgres-specific metrics. It requires a database user with the
                    <var>pg_monitor</var> role and the <var>pg_stat_statements</var> extension enabled.
                </p>
                <Code>
                    <pre>
create role coroot with login password '&lt;PASSWORD&gt;';
grant pg_monitor to coroot;
create extension pg_stat_statements;
                    </pre>
                </Code>
                <p>The <var>pg_stat_statements</var> extension should be loaded via the <var>shared_preload_libraries</var> server setting.</p>
            </template>

            <template v-if="type === 'mysql'">
                <p>This integration allows Coroot to collect Mysql-specific metrics. It requires a Mysql user with the following permissions:</p>
                <Code>
                    <pre>
CREATE USER 'coroot'@'%' IDENTIFIED BY '&lt;PASSWORD&gt;';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'coroot'@'%';
                    </pre>
                </Code>
            </template>

            <template v-if="type === 'redis'">
                <p>This integration allows Coroot to collect Redis-specific metrics.</p>
            </template>

            <v-form v-if="config" v-model="valid">
                <!-- eslint-disable vue/no-mutating-props -->
                <div class="subtitle-1 mt-3">Port</div>
                <v-text-field v-model="config.port" outlined dense :rules="[$validators.notEmpty]" hide-details />

                <div v-if="types[type].username">
                    <div class="subtitle-1 mt-3">Username</div>
                    <v-text-field v-model="config.credentials.username" outlined dense hide-details />
                </div>
                <div v-if="types[type].password">
                    <div class="subtitle-1 mt-3">Password</div>
                    <v-text-field v-model="config.credentials.password" type="password" outlined dense hide-details />
                </div>

                <div v-if="type === 'postgres'">
                    <div class="subtitle-1 mt-3">SSL Mode</div>
                    <v-select
                        v-model="sslmode"
                        :items="['disable', 'require', 'verify-ca']"
                        outlined
                        dense
                        hide-details
                        :menu-props="{ offsetY: true }"
                    />
                </div>

                <div v-if="type === 'mysql'">
                    <div class="subtitle-1 mt-3">TLS</div>
                    <v-select
                        v-model="tls"
                        :items="['false', 'true', 'skip-verify', 'preferred']"
                        outlined
                        dense
                        hide-details
                        :menu-props="{ offsetY: true }"
                    />
                </div>

                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-4">
                    {{ error }}
                </v-alert>
                <v-alert v-if="message" color="green" outlined text class="mt-4">
                    {{ message }}
                </v-alert>
                <v-btn block color="primary" @click="save(false)" :disabled="!valid || loading" class="mt-3">Save</v-btn>
                <v-btn block v-if="active && !config.disabled" color="error" @click="save(true)" :disabled="loading" class="mt-3">
                    Disable the integration
                </v-btn>
                <!-- eslint-enable vue/no-mutating-props -->
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script>
import Code from '../components/Code.vue';

export default {
    props: {
        appId: String,
        type: String,
        active: Boolean,
    },

    components: { Code },

    data() {
        return {
            config: null,
            dialog: false,
            valid: false,
            loading: false,
            error: '',
            message: '',

            sslmode: 'disable',
            tls: 'false',
        };
    },

    watch: {
        dialog(v) {
            v && this.get();
        },
    },

    computed: {
        types() {
            return {
                postgres: { name: 'Postgres', username: true, password: true },
                redis: { name: 'Redis', username: false, password: true },
                memcached: { name: 'Memcached', username: false, password: false },
                mongodb: { name: 'MongoDB', username: true, password: true },
                mysql: { name: 'MySQL', username: true, password: true },
            };
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getInstrumentation(this.appId, this.type, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.config = data;
                if (this.type === 'postgres' && this.config.params && this.config.params['sslmode']) {
                    this.sslmode = this.config.params['sslmode'];
                }
            });
        },
        save(disable) {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = { ...this.config, disabled: disable };
            if (this.type === 'postgres') {
                form.params = { sslmode: this.sslmode };
            }
            if (this.type === 'mysql') {
                form.params = { tls: this.tls };
            }
            this.$api.saveInstrumentationSettings(this.appId, this.type, form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                this.message = 'Settings were successfully updated. The changes will take effect in a minute or two.';
                setTimeout(() => {
                    this.message = '';
                    this.dialog = false;
                }, 3000);
            });
        },
    },
};
</script>

<style scoped>
.gap {
    gap: 8px;
}
</style>
