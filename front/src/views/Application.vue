<template>
<div>
    <h1 class="text-h5 my-5">
        Applications / {{$api.appId(id).name}}
        <v-progress-linear v-if="loading" indeterminate color="green" />
    </h1>

    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
        {{error}}
    </v-alert>

    <div v-if="app">
        <AppMap v-if="app.app_map" :map="app.app_map" class="my-5" />

        <v-tabs v-if="app.reports && app.reports.length" height="40" show-arrows slider-size="2">
            <v-tab v-for="r in app.reports" :key="r.name" :to="{params: {report: r.name}, query: $route.query}">
                {{r.name}}
            </v-tab>
        </v-tabs>
        <Dashboard v-if="r" :name="r.name" :widgets="r.widgets" class="mt-3" />
    </div>
    <NoData v-else-if="!loading" />
</div>
</template>

<script>
import AppMap from "@/components/AppMap";
import Dashboard from "@/components/Dashboard";
import NoData from "@/components/NoData";

export default {
    props: {
        id: String,
        report: String,
    },

    components: {AppMap, Dashboard, NoData},

    data() {
        return {
            app: null,
            loading: false,
            error: '',
            r: null,
        }
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
                this.r = this.app.reports[0];
                return;
            }
            const r = this.app.reports.find((r) => r.name === this.report);
            if (!r) {
                this.$router.replace({params: {report: null}}).catch(err => err);
                return;
            }
            this.r = r;
        },
    },
};
</script>
