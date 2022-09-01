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

        <v-tabs v-if="app.dashboards && app.dashboards.length" height="40" show-arrows slider-size="2">
            <v-tab v-for="d in app.dashboards" :to="{params: {dashboard: d.name}, query: $route.query}">
                {{d.name}}
            </v-tab>
        </v-tabs>
        <Dashboard v-if="dash" :widgets="dash.widgets" class="mt-3" />
    </div>
</div>
</template>

<script>
import AppMap from "@/components/AppMap";
import Dashboard from "@/components/Dashboard";

export default {
    props: {
        id: String,
        dashboard: String,
    },

    components: {AppMap, Dashboard},

    data() {
        return {
            app: null,
            loading: false,
            error: '',
            dash: null,
        }
    },

    mounted() {
        this.get();
        this.$api.contextWatch(this, this.get);
    },

    watch: {
        id() {
            this.app = null;
            this.get();
        },
        dashboard() {
            this.showDash();
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
                this.showDash();
            });
        },
        showDash() {
            if (!this.app || !this.app.dashboards || !this.app.dashboards.length) {
                this.dash = null;
                return;
            }
            if (!this.dashboard) {
                this.dash = this.app.dashboards[0];
                return;
            }
            const dash = this.app.dashboards.find((d) => d.name === this.dashboard);
            if (!dash) {
                this.$router.replace({params: {dashboard: null}}).catch(err => err);
                return;
            }
            this.dash = dash;
        },
    },
};
</script>
