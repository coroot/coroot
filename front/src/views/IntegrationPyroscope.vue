<template>
    <v-form v-if="form" v-model="valid" ref="form" style="max-width: 800px">
        <div class="subtitle-1">Pyroscope URL</div>
        <div class="caption">
        </div>
        <v-text-field outlined dense v-model="form.url" :rules="[$validators.isUrl]" placeholder="http://pyroscope.example.com:4040" hide-details="auto" class="flex-grow-1" clearable />
        <v-checkbox v-model="form.tls_skip_verify" :disabled="!form.url || !form.url.startsWith('https')" label="Skip TLS verify" hide-details class="mt-1" />
        <div class="d-md-flex gap">
            <v-checkbox v-model="basic_auth" label="HTTP basic auth" class="mt-1" />
            <template v-if="basic_auth">
                <v-text-field outlined dense v-model="form.basic_auth.user" label="username"  />
                <v-text-field v-model="form.basic_auth.password" label="password" type="password" outlined dense />
            </template>
        </div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{error}}
        </v-alert>
        <v-alert v-if="message" color="green" outlined text>
            {{message}}
        </v-alert>
        <v-btn v-if="saved.url && !form.url" block color="error" @click="del" :loading="loading">Delete</v-btn>
        <v-btn v-else block color="primary" @click="save" :disabled="!form.url || !valid" :loading="loading">Test & Save</v-btn>
    </v-form>
</template>

<script>
export default {
    data() {
        return {
            form: null,
            basic_auth: false,
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
            this.$api.getIntegrations('pyroscope', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form = data;
                if (!this.form.basic_auth) {
                    this.form.basic_auth = {user: '', password: ''};
                    this.basic_auth = false;
                } else {
                    this.basic_auth = true;
                }
                this.saved = JSON.parse(JSON.stringify(this.form));
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = JSON.parse(JSON.stringify(this.form));
            if (!this.basic_auth) {
                form.basic_auth = null;
            }
            this.$api.saveIntegrations('pyroscope', 'save', form, (data, error) => {
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
            this.$api.saveIntegrations('pyroscope', 'del', null, (data, error) => {
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