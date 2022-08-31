<template>
<v-app>
    <v-app-bar app flat dark color="#080d1b">
        <v-container class="py-0 fill-height">
            <router-link :to="{name: 'index'}">
                <img src="/static/logo.svg" height="38" style="vertical-align: middle;">
            </router-link>
            <v-spacer />
            <Search v-if="$vuetify.breakpoint.mdAndUp && $route.params.projectId" />
            <v-spacer />
            <TimePicker v-if="$route.params.projectId" :small="$vuetify.breakpoint.xsOnly"/>
        </v-container>
    </v-app-bar>

    <v-main>
        <v-container>
            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                {{error}}
            </v-alert>
            <router-view />
        </v-container>
    </v-main>
</v-app>
</template>

<script>
import TimePicker from "@/components/TimePicker";
import Search from "@/components/Search";

export default {
    components: {Search, TimePicker},

    data() {
        return {
            loading: false,
            error: '',
        }
    },

    created() {
        this.getProjects();
    },

    methods: {
        getProjects() {
            this.loading = true;
            this.$api.getProjects((data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                const projects = data;
                if (this.$route.name === 'index') {
                    if (!projects || !projects.length) {
                        this.$router.replace({name: 'project_new'});
                        return;
                    }
                    this.$router.replace({name: 'overview', params: {projectId: projects[0].id}});
                }
            });
        }
    },
}
</script>

<style>
a {
    text-decoration: none !important;
}
.v-btn {
    text-transform: none !important;
    font-weight: normal !important;
    letter-spacing: inherit !important;
    font-size: inherit !important;
}
</style>
