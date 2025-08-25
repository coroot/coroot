<template>
    <v-dialog v-model="show" persistent max-width="600">
        <v-card>
            <v-card-title class="d-flex align-center">
                <v-icon color="primary" class="mr-2">mdi-cloud-outline</v-icon>
                <span>Supercharge Coroot with AI</span>
            </v-card-title>

            <v-card-text>
                <div class="mb-4">
                    <p class="text-body-1 mb-3">Get AI-powered root cause analysis with <strong>Coroot Cloud</strong>!</p>

                    <v-list dense class="mb-4">
                        <v-list-item>
                            <v-list-item-icon>
                                <v-icon color="green">mdi-check-circle</v-icon>
                            </v-list-item-icon>
                            <v-list-item-content> <strong>10 free investigations per month</strong> – analyze incidents with AI </v-list-item-content>
                        </v-list-item>
                        <v-list-item>
                            <v-list-item-icon>
                                <v-icon color="green">mdi-check-circle</v-icon>
                            </v-list-item-icon>
                            <v-list-item-content>
                                <strong>Automatic or manual</strong> – investigate incidents as they happen or on-demand
                            </v-list-item-content>
                        </v-list-item>
                        <v-list-item>
                            <v-list-item-icon>
                                <v-icon color="green">mdi-check-circle</v-icon>
                            </v-list-item-icon>
                            <v-list-item-content> <strong>Quick setup</strong> – connect in seconds with your Google account </v-list-item-content>
                        </v-list-item>
                    </v-list>
                </div>
            </v-card-text>

            <v-card-actions class="px-6 pb-6">
                <v-btn color="primary" :to="{ name: 'project_settings', params: { tab: 'cloud' } }" @click="dismiss">
                    <v-icon left>mdi-rocket-launch</v-icon>
                    Connect to Coroot Cloud
                </v-btn>
                <v-spacer />
                <v-menu offset-y>
                    <template #activator="{ on, attrs }">
                        <v-btn text v-bind="attrs" v-on="on">
                            Not now
                            <v-icon right>mdi-chevron-down</v-icon>
                        </v-btn>
                    </template>
                    <v-list dense>
                        <v-list-item @click="remindLater">
                            <v-list-item-icon>
                                <v-icon>mdi-clock-outline</v-icon>
                            </v-list-item-icon>
                            <v-list-item-content>
                                <v-list-item-title>Remind me in a week</v-list-item-title>
                            </v-list-item-content>
                        </v-list-item>
                        <v-list-item @click="dismissPermanently">
                            <v-list-item-icon>
                                <v-icon>mdi-close-circle-outline</v-icon>
                            </v-list-item-icon>
                            <v-list-item-content>
                                <v-list-item-title>Don't show again</v-list-item-title>
                            </v-list-item-content>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script>
const DISMISS_KEY = 'cloud-promo-dismissed';
const REMIND_LATER_KEY = 'cloud-promo-remind-later';

export default {
    data() {
        return {
            dismissed: false,
            cloudStatus: '',
        };
    },

    mounted() {
        if (!this.shouldShow()) {
            this.dismissed = true;
        } else {
            this.getCloudStatus();
        }
    },

    computed: {
        show: {
            get() {
                return !this.dismissed && this.shouldShow() && this.cloudStatus && this.cloudStatus !== 'configured';
            },
            set(value) {
                if (!value) {
                    this.dismiss();
                }
            },
        },
    },

    methods: {
        shouldShow() {
            if (this.$route.name === 'project_settings' && this.$route.params.tab === 'cloud') {
                return false;
            }

            const permanentDismiss = this.$storage.local(DISMISS_KEY);
            if (permanentDismiss) {
                return false;
            }

            const remindLaterTime = this.$storage.local(REMIND_LATER_KEY);
            if (remindLaterTime) {
                const now = new Date().getTime();
                const reminderTime = parseInt(remindLaterTime);
                if (now < reminderTime) {
                    return false;
                }
                this.$storage.local(REMIND_LATER_KEY, null);
            }

            return true;
        },

        getCloudStatus() {
            this.$api.get('cloud', { query: 'status' }, (data, error) => {
                if (error) {
                    return;
                }
                this.cloudStatus = data.status || '';
            });
        },

        dismiss() {
            this.dismissed = true;
        },

        remindLater() {
            this.dismissed = true;
            const oneWeekFromNow = new Date().getTime() + 7 * 24 * 60 * 60 * 1000; // 7 days in milliseconds
            this.$storage.local(REMIND_LATER_KEY, oneWeekFromNow.toString());
        },

        dismissPermanently() {
            this.dismissed = true;
            this.$storage.local(DISMISS_KEY, true);
        },
    },
};
</script>

<style scoped>
.v-list-item__icon {
    margin-right: 12px !important;
}
</style>
