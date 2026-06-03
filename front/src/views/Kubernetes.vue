<template>
    <div>
        <Views :loading="loading" :error="error" />

        <v-tabs :value="tab" height="40" show-arrows slider-size="2" class="px-4 pb-2">
            <v-tab v-for="t in tabs" :key="t.id" :to="{ params: { id: t.id } }" :tab-value="t.id" exact>
                {{ t.name }}
                <span v-if="t.issues" class="issues-badge">{{ t.issues }}</span>
            </v-tab>
        </v-tabs>

        <template v-if="!error">
            <template v-if="!tab">
                <div class="pt-4">
                    <Logs
                        :show-sources="false"
                        :default-filters="[{ name: 'service.name', op: '=', value: 'KubernetesEvents' }]"
                        :hidden-attributes="['service.name']"
                        :columns="[
                            { key: 'date', label: 'Date' },
                            { key: 'cluster', label: 'Cluster', maxWidth: 20 },
                            { key: 'object.namespace', label: 'Namespace', maxWidth: 20 },
                            { key: 'object.name', label: 'Object', maxWidth: 20 },
                            { key: 'object.kind', label: 'Kind' },
                            { key: 'message', label: 'Message' },
                        ]"
                        @loading="setLoading"
                        @error="setError"
                    />
                </div>
            </template>
            <template v-else-if="tab === 'fluxcd'">
                <div class="pt-4">
                    <FluxCD @loading="setLoading" @error="setError" />
                </div>
            </template>
            <template v-else-if="tab === 'argocd'">
                <div class="pt-4">
                    <ArgoCD @loading="setLoading" @error="setError" />
                </div>
            </template>
            <template v-else-if="tab === 'rollouts'">
                <div class="pt-4">
                    <Deployments @loading="setLoading" @error="setError" />
                </div>
            </template>
        </template>
    </div>
</template>

<script>
import Views from '@/views/Views.vue';
import Deployments from '@/views/Deployments.vue';
import Logs from '@/components/Logs.vue';
import FluxCD from '@/components/FluxCD.vue';
import ArgoCD from '@/components/ArgoCD.vue';

export default {
    components: { Deployments, Views, Logs, FluxCD, ArgoCD },
    data() {
        return {
            tab: this.$route.params.id,
            error: '',
            loading: false,
        };
    },
    mounted() {
        const known = [undefined, 'fluxcd', 'argocd', 'rollouts'];
        if (this.$route.params.id && !known.includes(this.$route.params.id)) {
            this.$router.replace({ params: { id: undefined } });
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
            const ctx = this.$api.context || {};
            const flux = ctx.fluxcd;
            const argo = ctx.argocd;
            const tabs = [{ id: undefined, name: 'Events' }];
            if (flux || this.tab === 'fluxcd') {
                tabs.push({ id: 'fluxcd', name: 'FluxCD', issues: flux && flux.issues });
            }
            if (argo || this.tab === 'argocd') {
                tabs.push({ id: 'argocd', name: 'ArgoCD', issues: argo && argo.issues });
            }
            tabs.push({ id: 'rollouts', name: 'Rollouts' });
            return tabs;
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

<style scoped>
.issues-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    box-sizing: border-box;
    min-width: 16px;
    height: 16px;
    margin-left: 6px;
    padding: 0 4px;
    border-radius: 8px;
    font-size: 11px !important;
    line-height: 1;
    letter-spacing: 0 !important;
    color: white;
    background-color: #ffa726;
}
</style>
