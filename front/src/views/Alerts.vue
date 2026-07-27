<template>
    <div>
        <Views :loading="loading" :error="error" />

        <v-tabs :value="tab" height="40" show-arrows slider-size="2" class="px-4 pb-2">
            <v-tab v-for="t in tabs" :key="t.id" :to="{ params: { id: t.id } }" :tab-value="t.id" exact>
                {{ t.name }}
            </v-tab>
        </v-tabs>

        <template v-if="!error">
            <template v-if="!tab">
                <div class="pt-4">
                    <AlertsList @loading="setLoading" @error="setError" />
                </div>
            </template>
            <template v-else-if="tab === 'rules'">
                <div class="pt-4">
                    <AlertingRules @loading="setLoading" @error="setError" />
                </div>
            </template>
            <template v-else-if="tab === 'inspections'">
                <div class="pt-4">
                    <Inspections @loading="setLoading" @error="setError" />
                </div>
            </template>
        </template>
    </div>
</template>

<script>
import Views from '@/views/Views.vue';
import AlertsList from '@/components/AlertsList.vue';
import AlertingRules from '@/components/AlertingRules.vue';
import Inspections from '@/components/Inspections.vue';

export default {
    components: { Views, AlertsList, AlertingRules, Inspections },
    data() {
        return {
            tab: this.$route.params.id,
            error: '',
            loading: false,
        };
    },
    mounted() {
        this.syncTabRoute();
    },
    watch: {
        '$route.params.id'(newId) {
            this.tab = newId;
            this.error = '';
            this.syncTabRoute();
        },
        '$root.user'() {
            this.syncTabRoute();
        },
    },
    computed: {
        tabs() {
            const tabs = [{ id: undefined, name: 'Alerts' }];
            const menu = this.$root.user && this.$root.user.menu;
            if (menu && menu.alerting_rules) {
                tabs.push({ id: 'rules', name: 'Alerting Rules' });
            }
            // Inspections stay project-wide; namespace-scoped handoff users only
            // get Alerting Rules for their apps.
            if (menu && menu.inspections) {
                tabs.push({ id: 'inspections', name: 'Inspections' });
            }
            return tabs;
        },
        isHandoffUser() {
            const email = (this.$root.user && this.$root.user.email) || '';
            return email.endsWith('@handoff.local');
        },
    },

    methods: {
        syncTabRoute() {
            if (!this.$root.user) {
                return;
            }
            const currentId = this.$route.params.id;
            if (currentId && !this.tabs.find((t) => t.id === currentId)) {
                this.$router.replace({ params: { id: undefined } }).catch(() => {});
                return;
            }
            // Handoff users manage namespace-scoped rules; land on that tab so
            // "Add rule" is visible without hunting for a second tab.
            const menu = this.$root.user.menu;
            if (!currentId && menu && menu.alerting_rules && this.isHandoffUser) {
                this.$router.replace({ params: { id: 'rules' } }).catch(() => {});
            }
        },
        setLoading(loading) {
            this.loading = loading;
        },
        setError(error) {
            this.error = error;
        },
    },
};
</script>

<style scoped></style>
