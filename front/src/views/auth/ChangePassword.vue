<template>
    <v-dialog v-model="dialog" max-width="600">
        <v-card class="pa-4">
            <div class="d-flex align-center font-weight-medium mb-4">
                Change password
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>
            <v-form v-model="valid" @submit.prevent="post" ref="form">
                <div class="font-weight-medium">Old password</div>
                <v-text-field outlined dense type="password" v-model="form.old_password" :rules="[$validators.notEmpty]" />

                <div class="font-weight-medium">New password</div>
                <v-text-field outlined dense type="password" v-model="form.new_password" :rules="[$validators.notEmpty]" />

                <div class="font-weight-medium">Confirm password</div>
                <v-text-field
                    outlined
                    dense
                    type="password"
                    v-model="confirm_password"
                    :rules="[$validators.notEmpty, (v) => v === form.new_password || 'passwords do not match']"
                />

                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                    {{ error }}
                </v-alert>
                <v-alert v-else-if="message" color="green" outlined text>
                    {{ message }}
                </v-alert>

                <div class="d-flex align-center">
                    <v-spacer />
                    <v-btn type="submit" :disabled="!valid" :loading="loading" color="primary" class="mt-5">Change</v-btn>
                </div>
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script>
export default {
    props: {
        value: Boolean,
    },

    data() {
        return {
            dialog: this.value,
            form: {
                old_password: '',
                new_password: '',
            },
            confirm_password: '',
            valid: false,
            error: '',
            message: '',
            loading: false,
        };
    },

    watch: {
        value(v) {
            this.dialog = v;
        },
        dialog(v) {
            this.$emit('input', v);
            if (v) {
                this.error = '';
                this.message = '';
                this.form.old_password = '';
                this.form.new_password = '';
                this.confirm_password = '';
                this.$refs.form && this.$refs.form.resetValidation();
            }
        },
    },

    methods: {
        post() {
            this.loading = true;
            this.error = '';
            this.message = '';
            this.$api.user(this.form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'User password changed.';
            });
        },
    },
};
</script>

<style scoped></style>
