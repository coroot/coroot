<template>
    <v-form v-if="form" v-model="valid" ref="form" style="max-width: 800px">
        <div class="subtitle-1">Clickhouse address</div>
        <div class="caption">
        </div>
        <v-text-field outlined dense v-model="form.addr" :rules="[$validators.isAddr]" placeholder="clickhouse:9000" hide-details="auto" class="flex-grow-1" clearable single-line />
        <v-checkbox v-model="auth" label="auth" hide-details class="my-2" />
        <div v-if="auth" class="d-flex gap my-2">
            <v-text-field v-model="form.auth.user" label="username" outlined dense hide-details single-line />
            <v-text-field v-model="form.auth.password" label="password" type="password" outlined dense hide-details single-line />
        </div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{error}}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{message}}
        </v-alert>
        <v-btn v-if="saved.url && !form.url" block color="error" @click="del" :loading="loading">Delete</v-btn>
        <v-btn v-else block color="primary" @click="save" :disabled="!form.addr || !valid" :loading="loading">Test & Save</v-btn>
    </v-form>
</template>

<script>
export default {
    data() {
        return {
            form: null,
            auth: false,
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
                if (!this.form.auth) {
                    this.form.auth = {user: '', password: ''};
                    this.auth = false;
                } else {
                    this.auth = true;
                }
                this.saved = JSON.parse(JSON.stringify(this.form));
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = JSON.parse(JSON.stringify(this.form));
            if (!this.auth) {
                form.auth = null;
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
    }
}
</script>

<style scoped>
.gap {
    gap: 16px;
}
</style>