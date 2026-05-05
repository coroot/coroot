<template>
    <div class="form">
        <div class="text-center">
            <img :src="`${$coroot.base_path}static/icon.svg`" alt=":~#" height="80" />
        </div>

        <h2 class="text-h4 my-5 text-center">Authorize MCP access</h2>

        <p class="text-center mb-2">
            <span class="font-weight-medium">{{ clientName }}</span>
            is requesting access to Coroot telemetry as
            <span class="font-weight-medium">{{ userName }}</span
            >.
        </p>
        <p class="grey--text caption text-center mb-8">
            It will be able to read data from any project you have permission to view, until access is revoked.
        </p>

        <form method="POST" :action="formAction" ref="form">
            <input v-for="(v, k) in formFields" :key="k" type="hidden" :name="k" :value="v" />
            <input type="hidden" name="decision" :value="decision" />

            <div class="d-flex justify-end" style="gap: 8px">
                <v-btn outlined :loading="submitting && decision === 'deny'" @click="submit('deny')">Cancel</v-btn>
                <v-btn color="primary" :loading="submitting && decision === 'allow'" @click="submit('allow')">Authorize</v-btn>
            </div>
        </form>
    </div>
</template>

<script>
const FORWARDED = ['client_id', 'redirect_uri', 'response_type', 'code_challenge', 'code_challenge_method', 'state', 'scope'];

export default {
    data() {
        return {
            decision: '',
            submitting: false,
        };
    },
    computed: {
        clientName() {
            return this.$route.query.client_name || 'An MCP client';
        },
        userName() {
            return this.$route.query.user_name || '';
        },
        formAction() {
            return this.$coroot.base_path + 'oauth/authorize';
        },
        formFields() {
            const out = {};
            for (const k of FORWARDED) {
                if (this.$route.query[k] !== undefined) {
                    out[k] = this.$route.query[k];
                }
            }
            return out;
        },
    },
    methods: {
        submit(decision) {
            this.decision = decision;
            this.submitting = true;
            this.$nextTick(() => this.$refs.form.submit());
        },
    },
};
</script>

<style scoped>
.form {
    max-width: 600px;
    margin: 100px auto;
    padding: 0 16px;
}
</style>
