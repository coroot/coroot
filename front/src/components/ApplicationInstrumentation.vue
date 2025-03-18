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
                <a :href="`https://docs.coroot.com/databases/${type}`" target="_blank" class="ml-2">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
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

            <template v-if="type === 'mongodb'">
                <p>This integration allows Coroot to collect MongoDB-specific metrics.</p>
            </template>

            <template v-if="type === 'memcached'">
                <p>This integration allows Coroot to collect Memcached-specific metrics.</p>
            </template>

            <p>To enable metric collection for this database, add Kubernetes annotations or manually enter credentials using the form below.</p>

            <v-tabs v-model="tab" height="40" slider-size="2" class="my-4">
                <v-tab><v-icon class="mr-1">mdi-kubernetes</v-icon>Kubernetes</v-tab>
                <v-tab><v-icon class="mr-1">mdi-pencil</v-icon>Manual Configuration</v-tab>
            </v-tabs>

            <v-tabs-items v-model="tab">
                <v-tab-item transition="none">
                    <p>
                        Coroot-cluster-agent automatically discovers and collects metrics from pods annotated with
                        <var>coroot.com/{{ type }}-scrape</var> annotations.
                    </p>
                    <v-alert color="primary" outlined text>
                        Note that Coroot checks only <b>Pod</b> annotations, not higher-level Kubernetes objects like Deployments or StatefulSets.
                    </v-alert>
                    <Code>
                        <pre v-if="type === 'postgres'">
coroot.com/postgres-scrape: "true"
coroot.com/postgres-scrape-port: "5432"

# plain-text credentials
coroot.com/postgres-scrape-credentials-username: "coroot"
coroot.com/postgres-scrape-credentials-password: "&lt;PASSWORD&gt;"

# credentials from a secret
coroot.com/postgres-scrape-credentials-secret-name: "postgres-secret"
coroot.com/postgres-scrape-credentials-secret-username-key: "username"
coroot.com/postgres-scrape-credentials-secret-password-key: "password"

# client SSL options: disable, require, verify-ca (default: disable)
coroot.com/postgres-scrape-param-sslmode: "disable"
                        </pre>
                        <pre v-if="type === 'mysql'">
coroot.com/mysql-scrape: "true"
coroot.com/mysql-scrape-port: "3306"

# plain-text credentials
coroot.com/mysql-scrape-credentials-username: "coroot"
coroot.com/mysql-scrape-credentials-password: "&lt;PASSWORD&gt;"

# credentials from a secret
coroot.com/mysql-scrape-credentials-secret-name: "mysql-secret"
coroot.com/mysql-scrape-credentials-secret-username-key: "username"
coroot.com/mysql-scrape-credentials-secret-password-key: "password"

# client TLS options: true, false, skip-verify, preferred (default: false)
coroot.com/mysql-scrape-param-tls: "false"
                        </pre>
                        <pre v-if="type === 'redis'">
coroot.com/redis-scrape: "true"
coroot.com/redis-scrape-port: "6379"

# plain-text credentials
coroot.com/redis-scrape-credentials-password: "&lt;PASSWORD&gt;"

# credentials from a secret
coroot.com/redis-scrape-credentials-secret-name: "redis-secret"
coroot.com/redis-scrape-credentials-secret-password-key: "password"
                        </pre>
                        <pre v-if="type === 'mongodb'">
coroot.com/mongodb-scrape: "true"
coroot.com/mongodb-scrape-port: "27017"

# plain-text credentials
coroot.com/mongodb-scrape-credentials-username: "coroot"
coroot.com/mongodb-scrape-credentials-password: "&lt;PASSWORD&gt;"

# credentials from a secret
coroot.com/mongodb-scrape-credentials-secret-name: "mongodb-secret"
coroot.com/mongodb-scrape-credentials-secret-username-key: "username"
coroot.com/mongodb-scrape-credentials-secret-password-key: "password"
                        </pre>
                        <pre v-if="type === 'memcached'">
coroot.com/memcached-scrape: "true"
coroot.com/memcached-scrape-port: "11211"
                        </pre>
                    </Code>
                </v-tab-item>

                <v-tab-item transition="none">
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

                        <v-checkbox v-model="config.enabled" label="Enabled" dense hide-details class="my-3" />

                        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-4">
                            {{ error }}
                        </v-alert>
                        <v-alert v-if="message" color="green" outlined text class="mt-4">
                            {{ message }}
                        </v-alert>
                        <v-btn block color="primary" @click="save(true)" :disabled="loading || !valid" class="mt-3">Save</v-btn>
                        <!-- eslint-enable vue/no-mutating-props -->
                    </v-form>
                </v-tab-item>
            </v-tabs-items>
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

            tab: null,

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
                mysql: { name: 'MySQL', username: true, password: true },
                redis: { name: 'Redis', username: false, password: true },
                mongodb: { name: 'MongoDB', username: true, password: true },
                memcached: { name: 'Memcached', username: false, password: false },
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
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = { ...this.config };
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
