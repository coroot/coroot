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
        if (!this.tabs.find((t) => t.id === this.tab)) {
            if (this.$route.params.id) {
                this.$router.replace({ params: { id: undefined } });
            }
        }
    },
    watch: {
        '$route.params.id'(newId) {
            this.tab = newId;
            this.error = '';
        },
    },
    computed: {
        tabs() {
            return [
                { id: undefined, name: 'Alerts' },
                { id: 'rules', name: 'Alerting Rules' },
                { id: 'inspections', name: 'Inspections' },
            ];
        },
    },

    methods: {
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
