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
