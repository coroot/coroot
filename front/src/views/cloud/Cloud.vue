<template>
    <div style="max-width: 800px">
        <h1 class="text-h5 my-5">
            Coroot Cloud integration
            <v-progress-circular v-if="loading" indeterminate color="success" size="24" width="2" class="ml-2" />
        </h1>

        <v-alert color="primary" outlined text>
            Supercharge your Coroot Community Edition with AI-powered root cause analysis. Connect to Coroot Cloud and get intelligent insights that
            automatically investigate incidents for you. Start with 10 free credits per month – each credit covers one complete root cause analysis,
            so you can investigate up to 10 incidents at no cost.
        </v-alert>

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <div v-if="form.api_key">
            <div class="text-h6 mt-5">Connection Status</div>
            <div class="d-flex align-center mt-2 mb-3">
                <v-icon color="success" class="mr-2">mdi-check-circle</v-icon>
                <span class="font-weight-medium">Connected to Coroot Cloud</span>
                <v-spacer />
                <v-btn icon color="error" @click="confirmDisconnect">
                    <v-icon>mdi-link-off</v-icon>
                </v-btn>
            </div>

            <div class="text-h6 mt-3">API Key</div>
            <div class="caption mb-2">
                Your Coroot Cloud API key. Use it in the Coroot config file for IaC, or directly in the Coroot Custom Resource.
            </div>
            <div class="d-flex align-center" style="gap: 8px">
                <v-text-field :value="form.api_key" outlined dense readonly type="password" hide-details />
                <CopyButton :text="form.api_key" />
            </div>

            <div v-if="rca" class="text-h6 mt-5">Billing & Usage</div>
            <div v-if="rca" class="mt-2">
                <v-row>
                    <v-col cols="12" sm="6">
                        <div class="caption grey--text">Current Plan</div>
                        <div class="font-weight-medium">{{ rca.plan }}</div>

                        <div class="caption grey--text mt-2">Price</div>
                        <div class="font-weight-medium">${{ rca.price }} / {{ rca.interval }}</div>
                    </v-col>
                    <v-col cols="12" sm="6">
                        <div class="caption grey--text">Credits Usage</div>
                        <v-progress-linear
                            :value="(rca.credits_spent / rca.credits_total) * 100"
                            height="6"
                            rounded
                            color="primary"
                            class="mt-1 mb-1"
                        />
                        <div class="caption">{{ rca.credits_spent }} of {{ rca.credits_total }} credits used</div>

                        <div class="caption grey--text mt-2">Renews</div>
                        <div class="font-weight-medium">{{ $format.date(rca.renews_at * 1000, '{MMM} {DD}, {YYYY}') }}</div>
                    </v-col>
                </v-row>
            </div>

            <div class="text-h6 mt-4">Settings</div>
            <div class="caption mb-2">
                When enabled, Coroot will automatically investigate incidents as they are created. You can also trigger investigations manually when
                this setting is disabled.
            </div>
            <v-checkbox v-model="form.incidents_auto_investigation" label="Investigate incidents automatically" dense hide-details />

            <v-btn color="primary" @click="saveSettings" :loading="loading" :disabled="!changed" class="mt-3"> Save </v-btn>

            <v-dialog v-model="disconnectDialog" max-width="500">
                <v-card class="pa-2">
                    <v-card-title>
                        <v-icon color="warning" class="mr-2">mdi-alert-outline</v-icon>
                        Disconnect from Coroot Cloud?
                    </v-card-title>
                    <v-card-text>
                        <p class="mb-3">Are you sure you want to disconnect from Coroot Cloud? This will:</p>
                        <ul class="mb-3">
                            <li>Remove AI-powered root cause analysis</li>
                            <li>Stop automatic incident investigation</li>
                            <li>Require re-authentication to reconnect</li>
                        </ul>
                        <p class="mb-0">You can always reconnect later with the same account.</p>
                    </v-card-text>
                    <v-card-actions>
                        <v-spacer />
                        <v-btn text @click="disconnectDialog = false">Cancel</v-btn>
                        <v-btn color="error" @click="disconnect" :loading="loading"> Disconnect </v-btn>
                    </v-card-actions>
                </v-card>
            </v-dialog>
        </div>
        <div v-else-if="!loading">
            <Signup v-if="auth === 'signup'" @signin="auth = 'signin'" @google="google" />
            <Signin v-if="auth === 'signin'" @signup="auth = 'signup'" @google="google" @success="getAPIKey" />
        </div>
    </div>
</template>

<script>
import cloud from './api';
import Signup from './Signup.vue';
import Signin from './Signin.vue';
import CopyButton from '@/components/CopyButton.vue';

export default {
    components: { Signup, Signin, CopyButton },

    data() {
        return {
            loading: false,
            error: '',
            message: '',

            auth: 'signup',

            form: {
                api_key: '',
                incidents_auto_investigation: true,
            },
            saved: '',

            rca: null,

            disconnectDialog: false,
        };
    },

    mounted() {
        if (this.$route.query.t) {
            this.getAPIKey(this.$route.query.t);
            this.$router.replace({ query: { ...this.$route.query, t: undefined } }).catch((err) => err);
            return;
        }
        this.get();
    },

    computed: {
        changed() {
            return this.saved !== JSON.stringify(this.form);
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.get('cloud', {}, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form = data.form;
                this.saved = JSON.stringify(this.form);
                this.rca = data.info.rca;
            });
        },
        post() {
            this.loading = true;
            this.error = '';
            this.$api.post('cloud', this.form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.get();
            });
        },
        saveSettings() {
            this.post(this.form);
        },
        confirmDisconnect() {
            this.disconnectDialog = true;
        },
        disconnect() {
            this.disconnectDialog = false;
            this.form.api_key = '';
            this.post();
        },
        getAPIKey(token) {
            cloud.token = token;
            this.loading = true;
            cloud.get('/account/api_keys', {}, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                const api_key = data && data.length && data[0].key;
                if (!api_key) {
                    this.error = 'Failed to get API key.';
                    return;
                }
                this.form.api_key = api_key;
                this.post();
            });
        },
        google() {
            const req = {
                State: JSON.stringify({ return_url: window.location.href }),
                RedirectURL: cloud.url + '/auth/google',
            };
            this.loading = true;
            cloud.post('/auth/google', req, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                window.location.href = data;
            });
        },
    },
};
</script>

<style scoped></style>
