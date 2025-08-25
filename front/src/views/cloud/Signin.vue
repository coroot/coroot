<template>
    <div>
        <div class="text-center my-6">
            <div class="text-h5 font-weight-medium">Sign In to Coroot Cloud</div>
        </div>

        <v-form v-model="valid" @submit.prevent="submit" ref="form">
            <v-alert v-if="error" color="error" icon="mdi-alert-octagon-outline" outlined text>
                {{ error }}
            </v-alert>
            <v-alert v-else-if="message" color="success" outlined text>
                {{ message }}
            </v-alert>

            <div class="subtitle-1">Email</div>
            <v-text-field outlined dense type="email" v-model="form.Email" name="email" :rules="[$validators.notEmpty]" />

            <div class="subtitle-1">Password</div>
            <v-text-field outlined dense type="password" v-model="form.Password" name="password" :rules="[$validators.notEmpty]" />

            <a :href="reset" target="_blank">Forgot password?</a>

            <v-btn block type="submit" :disabled="!valid" :loading="loading" color="primary" class="mt-5">Sign In</v-btn>
        </v-form>

        <div class="my-5 text-center">or</div>

        <v-btn block @click="google" outlined>
            <img src="/static/img/icons/google.svg" height="20" alt="google icon" style="position: absolute; left: 0" />
            Sign In with Google
        </v-btn>

        <div class="mt-6 text-center">
            <span class="caption grey--text">Don't have an account? </span>
            <a @click="signup" class="caption font-weight-medium">Sign Up</a>
        </div>
    </div>
</template>

<script>
import cloud from './api';

export default {
    data() {
        return {
            loading: false,
            error: '',
            message: '',
            form: {
                Email: '',
                Password: '',
                ReturnURL: window.location.href,
            },
            valid: false,
        };
    },

    computed: {
        reset() {
            const email = encodeURIComponent(this.form.Email);
            const next = encodeURIComponent(window.location.href);
            return `${cloud.url}/auth/reset?email=${email}&next=${next}`;
        },
    },

    methods: {
        submit() {
            this.loading = true;
            this.error = '';
            this.message = '';
            cloud.post('/auth/signin', this.form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                if (data.message) {
                    this.message = data.message;
                    return;
                }
                this.$emit('success', data);
            });
        },
        signup() {
            this.$emit('signup');
        },
        google() {
            this.$emit('google');
        },
    },
};
</script>

<style scoped></style>
