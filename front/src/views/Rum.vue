<template>
    <Views :loading="loading" :error="error">
        <v-alert v-if="!loading && rum && rum.status !== 'ok'" :type="rum.status === 'warning' ? 'warning' : 'info'" outlined text class="mb-4">
            <span v-html="rum.message" />
            <RumIntegration class="ml-2" small color="primary">Integrate RUM</RumIntegration>
        </v-alert>

        <div class="d-flex align-center mb-4">
            <div class="text-h6">Frontend (RUM) services</div>
            <v-spacer />
            <RumIntegration color="primary">Integrate RUM</RumIntegration>
        </div>

        <v-simple-table dense>
            <thead>
                <tr>
                    <th>Service</th>
                    <th>Application</th>
                    <th class="text-right">Page views</th>
                    <th class="text-right">Errors</th>
                    <th class="text-right">Fetch error %</th>
                    <th class="text-right">p75 Load</th>
                    <th class="text-right">p75 LCP</th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="s in (rum && rum.services) || []" :key="s.name">
                    <td>{{ s.name }}</td>
                    <td>
                        <router-link :to="{ name: 'overview', params: { view: 'rum', id: s.app_id, report: 'RUM' } }">
                            {{ s.app_id }}
                        </router-link>
                    </td>
                    <td class="text-right">{{ formatCount(s.page_views) }}</td>
                    <td class="text-right">{{ formatCount(s.error_count) }}</td>
                    <td class="text-right">{{ formatPct(s.fetch_error_pct) }}</td>
                    <td class="text-right">{{ formatMs(s.p75_load_ms) }}</td>
                    <td class="text-right">{{ formatMs(s.p75_lcp_ms) }}</td>
                </tr>
                <tr v-if="!loading && rum && !(rum.services || []).length">
                    <td colspan="7" class="grey--text">No RUM services yet</td>
                </tr>
            </tbody>
        </v-simple-table>

        <div class="mt-8 text-subtitle-1">Service Map includes RumClient nodes when browser → API edges are observed.</div>
        <router-link :to="{ name: 'overview', params: { view: 'map' } }">Open Service Map</router-link>
    </Views>
</template>

<script>
import Views from '@/views/Views.vue';
import RumIntegration from '@/views/RumIntegration.vue';

export default {
    components: { Views, RumIntegration },
    data() {
        return {
            loading: false,
            error: '',
            rum: null,
        };
    },
    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },
    methods: {
        formatCount(v) {
            if (v == null) return '—';
            return Number(v).toLocaleString();
        },
        formatMs(v) {
            if (v == null || v === 0 || Number.isNaN(Number(v))) return '—';
            return Math.round(Number(v)) + ' ms';
        },
        formatPct(v) {
            if (v == null || Number.isNaN(Number(v))) return '—';
            return Number(v).toFixed(1) + '%';
        },
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getOverview('rum', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.rum = (data && data.rum) || null;
            });
        },
    },
};
</script>
