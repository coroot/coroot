<template>
    <div style="max-width: 800px">
        <p>
            Coroot leverages Large Language Models (LLMs) to automatically generate clear, concise summaries of root causes, helping your team
            troubleshoot faster.
        </p>
        <v-alert v-if="disabled" color="info" outlined text>
            Available exclusively in Coroot Enterprise (from $1 per CPU core/month).<br />
            <a href="https://coroot.com/account" target="_blank" class="font-weight-bold">Start</a> your free trial today.
        </v-alert>
        <v-form v-if="form" v-model="valid" :disabled="disabled" ref="form">
            <div class="subtitle-1 mt-3">Model Provider</div>
            <v-radio-group v-model="form.provider" row dense class="mt-0" hide-details>
                <v-radio value="anthropic">
                    <template #label>
                        <img :src="`${$coroot.base_path}static/img/icons/anthropic.svg`" height="20" width="20" class="mr-1" />
                        Anthropic
                    </template>
                </v-radio>
                <v-radio value="openai">
                    <template #label>
                        <img :src="`${$coroot.base_path}static/img/icons/openai.svg`" height="20" width="20" class="mr-1" />
                        OpenAI
                    </template>
                </v-radio>
                <v-radio value="openai_compatible">
                    <template #label>
                        <v-icon class="mr-1">mdi-cog-outline</v-icon>
                        OpenAI-compatible API
                    </template>
                </v-radio>
                <v-radio value="">
                    <template #label>
                        <v-icon class="mr-1">mdi-trash-can-outline</v-icon>
                        Disabled
                    </template>
                </v-radio>
            </v-radio-group>

            <template v-if="form.provider === 'anthropic'">
                <div class="subtitle-1 mt-3">API Key</div>
                <div class="caption">
                    To integrate Coroot with Anthropic models, provide your Anthropic API key. You can get started by following the
                    <a href="https://docs.anthropic.com/en/api/getting-started" target="_blank">official Anthropic API documentation</a>.
                </div>
                <v-text-field
                    v-model="form.anthropic.api_key"
                    :rules="[$validators.notEmpty]"
                    outlined
                    dense
                    hide-details
                    single-line
                    type="password"
                />
            </template>

            <template v-if="form.provider === 'openai'">
                <div class="subtitle-1 mt-3">API Key</div>
                <div class="caption">
                    To integrate Coroot with OpenAI models, provide your OpenAI API key. Learn more about the API on the
                    <a href="https://openai.com/index/openai-api/" target="_blank">OpenAI API overview page</a>.
                </div>
                <v-text-field v-model="form.openai.api_key" :rules="[$validators.notEmpty]" outlined dense hide-details single-line type="password" />
            </template>

            <template v-if="form.provider === 'openai_compatible'">
                <div class="subtitle-1 mt-3">Base URL</div>
                <div class="caption">
                    The base URL for API requests to the model provider. Refer to their documentation for configuration details.
                </div>
                <v-text-field v-model="form.openai_compatible.base_url" :rules="[$validators.isUrl]" outlined dense hide-details single-line />

                <div class="subtitle-1 mt-3">API Key</div>
                <div class="caption">To integrate Coroot with an OpenAI-compatible model, provide your API key.</div>
                <v-text-field
                    v-model="form.openai_compatible.api_key"
                    :rules="[$validators.notEmpty]"
                    outlined
                    dense
                    hide-details
                    single-line
                    type="password"
                />

                <div class="subtitle-1 mt-3">Model</div>
                <div class="caption">The name or ID of the model to use. Refer to your provider’s documentation for valid values.</div>
                <v-text-field v-model="form.openai_compatible.model" :rules="[$validators.notEmpty]" outlined dense hide-details single-line />
            </template>

            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="mt-3">
                {{ error }}
            </v-alert>
            <v-alert v-if="message" color="green" outlined text class="mt-3">
                {{ message }}
            </v-alert>
            <div class="mt-3 d-flex" style="gap: 8px">
                <v-btn color="primary" @click="save" :disabled="disabled || !valid || !changed" :loading="loading">Save</v-btn>
            </div>
        </v-form>
    </div>
</template>

<script>
export default {
    components: {},

    data() {
        return {
            disabled: this.$coroot.edition !== 'Enterprise',
            form: { provider: '', anthropic: {}, openai: {}, openai_compatible: {} },
            valid: false,
            loading: false,
            error: '',
            message: '',
            saved: {},
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
            this.$api.ai(null, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.form.provider = data.provider;
                this.form.anthropic = data.anthropic || {};
                this.form.openai = data.openai || {};
                this.form.openai_compatible = data.openai_compatible || {};
                this.saved = JSON.parse(JSON.stringify(this.form));
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = JSON.parse(JSON.stringify(this.form));
            this.$api.ai(form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                }, 3000);
                this.get();
            });
        },
    },
};
</script>

<style scoped></style>
