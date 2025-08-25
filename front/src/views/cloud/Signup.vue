<template>
    <div>
        <div class="text-center my-6">
            <div class="text-h5 font-weight-medium">Join Coroot Cloud</div>
        </div>

        <v-form v-model="valid" @submit.prevent="submit" ref="form">
            <v-alert v-if="error" color="error" icon="mdi-alert-octagon-outline" outlined text>
                {{ error }}
            </v-alert>
            <v-alert v-else-if="message" color="success" outlined text>
                {{ message }}
            </v-alert>

            <v-row no-gutters class="gap-2">
                <v-col>
                    <div class="subtitle-1">First name</div>
                    <v-text-field outlined dense v-model="form.FirstName" :rules="[$validators.notEmpty]" />
                </v-col>
                <v-col>
                    <div class="subtitle-1">Last name</div>
                    <v-text-field outlined dense v-model="form.LastName" :rules="[$validators.notEmpty]" />
                </v-col>
            </v-row>

            <div class="subtitle-1">Email</div>
            <v-text-field outlined dense type="email" v-model="form.Email" name="email" :rules="[$validators.notEmpty]" />

            <div class="subtitle-1">Password</div>
            <v-text-field outlined dense type="password" v-model="form.Password" name="password" :rules="[$validators.isPassword]" />

            <p>
                By signing up, you agree to the
                <a href="https://coroot.com/terms" target="_blank">Terms of Service</a>
                and
                <a href="https://coroot.com/privacy" target="_blank">Privacy Policy</a>.
            </p>

            <v-btn block type="submit" :disabled="!valid" :loading="loading" color="primary" class="mt-5">Sign Up</v-btn>
        </v-form>

        <div class="my-5 text-center">or</div>

        <v-btn block @click="google" outlined>
            <img src="/static/img/icons/google.svg" height="20" alt="google icon" style="position: absolute; left: 0" />
            Sign Up with Google
        </v-btn>

        <div class="mt-6 text-center">
            <span class="caption grey--text">Already have an account? </span>
            <a @click="signin" class="caption font-weight-medium">Sign In</a>
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
                FirstName: '',
                LastName: '',
                Email: '',
                Password: '',
                ReturnURL: window.location.href,
            },
            valid: false,
        };
    },

    methods: {
        submit() {
            this.loading = true;
            this.error = '';
            this.message = '';
            cloud.post('/auth/signup', this.form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = data;
            });
        },
        signin() {
            this.$emit('signin');
        },
        google() {
            this.$emit('google');
        },
    },
};
</script>

<style scoped></style>
