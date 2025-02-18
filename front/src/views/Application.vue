<template>
    <div>
        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <h1 v-else class="text-h5">
            {{ $utils.appId(id).name }}
            <v-progress-linear v-if="loading" indeterminate color="green" />
        </h1>

        <div v-if="app">
            <AppMap v-if="app.app_map" :map="app.app_map" class="my-5" />

            <v-tabs v-if="app.reports && app.reports.length" height="40" show-arrows slider-size="2">
                <v-tab v-for="r in app.reports" :key="r.name" :to="{ params: { report: r.name }, query: $utils.contextQuery() }" exact-path>
                    <Led v-if="r && (r.checks || r.instrumentation)" :status="r.status" />
                    {{ r.name }}
                </v-tab>
            </v-tabs>

            <v-card v-if="r && !r.custom && (r.checks || r.instrumentation)" outlined class="my-4 pa-4 pb-2">
                <ApplicationInstrumentation v-if="r.instrumentation" :appId="id" :type="r.instrumentation" :active="r.status !== 'unknown'" />
                <Check v-for="check in r.checks" :key="check.id" :appId="id" :check="check" class="mb-2" />
            </v-card>

            <Dashboard v-if="r" :name="r.name" :widgets="r.widgets" />
        </div>
        <NoData v-else-if="!loading && !error" />
    </div>
</template>

<script>
import AppMap from '../components/AppMap';
import Dashboard from '../components/Dashboard';
import NoData from '../components/NoData';
import Check from '../components/Check';
import Led from '../components/Led';
import ApplicationInstrumentation from '../components/ApplicationInstrumentation.vue';

export default {
    props: {
        id: String,
        report: String,
    },

    components: { AppMap, Dashboard, NoData, Check, Led, ApplicationInstrumentation },

    data() {
        return {
            app: null,
            loading: false,
            error: '',
            r: null,
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        id() {
            this.app = null;
            this.get();
        },
        report() {
            this.showReport();
        },
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getApplication(this.id, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.app = data;
                this.showReport();
            });
        },
        showReport() {
            if (!this.app || !this.app.reports || !this.app.reports.length) {
                this.r = null;
                return;
            }
            if (!this.report) {
                if (this.app.reports.length > 1 && this.app.reports[0].name === 'SLO' && this.app.reports[0].status === 'unknown') {
                    this.r = this.app.reports[1];
                } else {
                    this.r = this.app.reports[0];
                }
                this.$router.replace({ params: { report: this.r.name }, query: this.$utils.contextQuery() }).catch((err) => err);
                return;
            }
            const r = this.app.reports.find((r) => r.name === this.report);
            if (!r) {
                this.$router.replace({ params: { report: null }, query: this.$utils.contextQuery() }).catch((err) => err);
                return;
            }
            this.r = r;
        },
    },
};
</script>
