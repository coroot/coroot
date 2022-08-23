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
        <AppMap :application="app.application" :instances="app.instances" :clients="app.clients" :dependencies="app.dependencies" class="my-5" />

        <v-tabs v-if="app.dashboards && app.dashboards.length">
            <template v-for="d in app.dashboards">
                <v-tab>
                    {{d.name}}
                </v-tab>
                <v-tab-item transition="none">
                    <Widget v-for="w in d.widgets" :w="w" />
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
    },

    watch: {
        id() {
            this.app = null;
            this.get();
        }
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
