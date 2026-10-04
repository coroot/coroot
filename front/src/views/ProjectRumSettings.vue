<template>
    <div style="max-width: 800px">
        <h2 class="text-h5 mt-10 mb-5">RUM data retention</h2>
        <p>
            Control how long Real User Monitoring data is kept in ClickHouse. Leave a field empty to use the global default. Aggregates (dashboards)
            can be retained longer than raw page views and sessions.
        </p>
        <v-form v-if="form" v-model="valid" :disabled="!editable" @submit.prevent="save">
            <div class="subtitle-1">Raw data TTL</div>
            <div class="caption mb-1">
                Page views, spans, events (default {{ defaults.raw }}). Used for explorer, sessions, and tracing drill-down.
            </div>
            <v-text-field
                v-model="form.raw_ttl"
                :placeholder="defaults.raw"
                :rules="[durationRule]"
                outlined
                dense
                hide-details="auto"
                class="mb-4"
            />

            <div class="subtitle-1">Session replay TTL</div>
            <div class="caption mb-1">DOM replay segments (default {{ defaults.replay }}). Heaviest RUM table — keep short unless needed.</div>
            <v-text-field
                v-model="form.replay_ttl"
                :placeholder="defaults.replay"
                :rules="[durationRule]"
                outlined
                dense
                hide-details="auto"
                class="mb-4"
            />

            <div class="subtitle-1">Aggregates TTL</div>
            <div class="caption mb-1">Per-minute rollups and histograms for dashboards (default {{ defaults.aggregates }}). Must be ≥ raw TTL.</div>
            <v-text-field
                v-model="form.aggregates_ttl"
                :placeholder="defaults.aggregates"
                :rules="[durationRule, aggregatesGteRaw]"
                outlined
                dense
                hide-details="auto"
                class="mb-4"
            />

            <v-switch v-model="form.geo_enabled" label="Enrich with geo.country from client IP" dense class="mt-0" />
            <v-switch v-model="form.replay_enabled" label="Enable session replay ingest" dense class="mt-0" />
            <div v-if="form.replay_enabled" class="caption mb-2">Replay sample rate (0–1). At 0.1, roughly 10% of consented sessions are stored.</div>
            <v-text-field
                v-if="form.replay_enabled"
                v-model.number="form.replay_sample_rate"
                type="number"
                min="0"
                max="1"
                step="0.01"
                outlined
                dense
                hide-details="auto"
                class="mb-4"
                style="max-width: 200px"
            />

            <v-alert v-if="error" color="red" outlined text class="mt-2">{{ error }}</v-alert>
            <v-alert v-if="message" color="green" outlined text class="mt-2">{{ message }}</v-alert>
            <v-btn color="primary" :disabled="!editable || !valid" :loading="loading" @click="save">Save</v-btn>
        </v-form>
    </div>
</template>

<script>
export default {
    data() {
        return {
            loading: false,
            valid: false,
            editable: false,
            error: '',
            message: '',
            form: null,
            defaults: { raw: '7d', replay: '7d', aggregates: '30d' },
        };
    },
    mounted() {
        this.get();
    },
    methods: {
        durationRule(v) {
            if (!v) return true;
            return /^\d+[smhdw]$/.test(String(v).trim()) || 'Use a duration like 7d, 24h, 30d';
        },
        aggregatesGteRaw() {
            const raw = this.parseDays(this.form && this.form.raw_ttl) || this.parseDays(this.defaults.raw);
            const agg = this.parseDays(this.form && this.form.aggregates_ttl) || this.parseDays(this.defaults.aggregates);
            if (raw && agg && agg < raw) return 'Aggregates TTL must be ≥ raw TTL';
            return true;
        },
        parseDays(s) {
            if (!s) return 0;
            const m = String(s)
                .trim()
                .match(/^(\d+)([smhdw])$/);
            if (!m) return 0;
            const n = Number(m[1]);
            switch (m[2]) {
                case 's':
                    return n / 86400;
                case 'm':
                    return n / 1440;
                case 'h':
                    return n / 24;
                case 'd':
                    return n;
                case 'w':
                    return n * 7;
                default:
                    return 0;
            }
        },
        get() {
            this.$api.get(`project/${this.$route.params.projectId}/rum_settings`, {}, (data, err) => {
                if (err) {
                    this.error = err;
                    return;
                }
                this.editable = !!data.editable;
                this.defaults = {
                    raw: data.default_raw_ttl || '7d',
                    replay: data.default_replay_ttl || '7d',
                    aggregates: data.default_aggregates_ttl || '30d',
                };
                this.form = {
                    geo_enabled: !!data.geo_enabled,
                    replay_enabled: !!data.replay_enabled,
                    replay_sample_rate: data.replay_sample_rate || 0,
                    raw_ttl: data.raw_ttl || '',
                    replay_ttl: data.replay_ttl || '',
                    aggregates_ttl: data.aggregates_ttl || '',
                };
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            this.$api.post(`project/${this.$route.params.projectId}/rum_settings`, this.form, (data, err) => {
                this.loading = false;
                if (err) {
                    this.error = err;
                    return;
                }
                this.message = 'Saved. Retention will be applied shortly.';
                this.get();
            });
        },
    },
};
</script>
