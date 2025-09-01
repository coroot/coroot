<template>
    <div>
        <v-alert app v-if="show" light color="amber lighten-4" :height="height" tile dismissible class="ma-0 py-1">
            <template #close>
                <v-btn x-small icon @click="dismiss"><v-icon small>mdi-close</v-icon></v-btn>
            </template>
            <div v-html="license.message" class="text-center" />
        </v-alert>

        <v-dialog :value="license.invalid" persistent max-width="600" no-click-animation>
            <v-card>
                <v-card-title class="d-flex align-center red--text">
                    <v-icon color="red" class="mr-2" large>mdi-alert-circle</v-icon>
                    <span>Invalid License</span>
                </v-card-title>

                <v-card-text class="pb-0">
                    <div class="mb-4">
                        <p class="text-body-1 mb-3">
                            <strong>Your Coroot Enterprise license is invalid or has expired.</strong>
                        </p>

                        <p class="text-body-2 mb-3">
                            Coroot Enterprise cannot run without a valid license. Please update your license or contact your administrator to restore
                            access.
                        </p>
                    </div>
                </v-card-text>

                <v-card-actions class="px-6 pb-6">
                    <v-btn color="primary" href="https://coroot.com/account" target="_blank" block> Manage you licenses in Customer Portal </v-btn>
                </v-card-actions>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
const key = 'license-alert-dismissed';

export default {
    props: {
        height: Number,
    },

    data() {
        return {
            context: this.$api.context,
            dismissed: !!this.$storage.local(key),
        };
    },

    computed: {
        license() {
            return this.context.license;
        },
        show() {
            return !this.dismissed && !!this.license.message;
        },
    },

    watch: {
        show(v) {
            this.$emit('show', v);
        },
    },

    methods: {
        dismiss() {
            this.dismissed = true;
            this.$storage.local(key, this.license.message);
        },
    },
};
</script>

<style scoped></style>
