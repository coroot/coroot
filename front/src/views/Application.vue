<template>
<div>
    <h1 class="text-h5 my-5">
        <router-link :to="{name: 'overview', query: $route.query}">Applications</router-link>
        / {{$api.appId(id).name}}
        <v-progress-linear v-if="loading" indeterminate color="green" />
    </h1>

    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
        {{error}}
    </v-alert>

    <div v-if="app">
        <AppMap v-if="app.app_map" :map="app.app_map" class="my-5" />

        <v-tabs v-if="app.dashboards && app.dashboards.length">
            <template v-for="d in app.dashboards">
                <v-tab>
                    {{d.name}}
                </v-tab>
                <v-tab-item transition="none">
                    <div class="d-flex flex-wrap">
                        <Widget v-for="w in d.widgets" :w="w" class="my-5" :style="{width: $vuetify.breakpoint.mdAndUp ? (w.width || '50%') : '100%'}" />
                    </div>
                </v-tab-item>
            </template>
        </v-tabs>
    </div>
</div>
</template>

<script>
import AppMap from "@/components/AppMap";
import Widget from "@/components/Widget";

export default {
    props: {
        id: String,
    },

    components: {AppMap, Widget},

    data() {
        return {
            app: null,
            loading: false,
            error: '',
        }
    },

    mounted() {
        this.get();
        this.$api.timeContextWatch(this, this.get);
    },

    watch: {
        id() {
            this.app = null;
            this.get();
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
            });
        }
    },
};
</script>