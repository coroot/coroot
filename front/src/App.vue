<template>
<v-app>
    <v-app-bar app flat dark color="#080d1b" class="menu">
        <v-container class="py-0 fill-height flex-nowrap">
            <router-link :to="project ? {name: 'overview', query: $route.query} : {name: 'index'}">
                <img src="/static/logo.svg" height="38" style="vertical-align: middle;">
            </router-link>

            <v-menu v-if="projects && projects.length" dark offset-y tile>
                <template #activator="{ on }">
                    <v-btn v-on="on" plain class="ml-3 px-1">
                        <v-icon small class="mr-2">mdi-hexagon-multiple</v-icon>
                        <span class="project-name">
                            <template v-if="project">{{project.name}}</template>
                            <i v-else>new project</i>
                        </span>
                    </v-btn>
                </template>
                <v-list dense color="#080d1b">
                    <v-list-item v-for="p in projects" :to="{name: 'overview', params: {projectId: p.id}}">
                        {{p.name}}
                    </v-list-item>
                    <v-list-item :to="{name: 'project_new'}" exact>
                        <v-icon small>mdi-plus</v-icon> new project
                    </v-list-item>
                </v-list>
            </v-menu>

            <v-spacer />

            <Search v-if="$vuetify.breakpoint.mdAndUp && project" />

            <v-spacer />

            <TimePicker v-if="project && $route.name !== 'project_settings'" :small="$vuetify.breakpoint.xsOnly"/>

            <v-btn v-if="project" icon small :to="{name: 'project_settings', params: {projectId: project.id}}" plain>
                <v-icon>mdi-cog</v-icon>
            </v-btn>
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
            projects: [],
            loading: false,
            error: '',
        }
    },

    created() {
        this.getProjects();
        this.$root.$on('project-saved', this.getProjects);
    },

    computed: {
        project() {
            const id = this.$route.params.projectId;
            if (!id) {
                return null;
            }
            return this.projects.find((p) => p.id === id);
        }
    },

    watch: {
        '$route.params.projectId': {
            handler: function(newValue) {
                this.lastProject(newValue);
            },
            immediate: true,
        }
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
                this.projects = data || [];
                if (this.$route.name === 'index') {
                    if (!this.projects.length) {
                        this.$router.replace({name: 'project_new'});
                        return;
                    }
                    let id = this.projects[0].id;
                    const lastId = this.lastProject();
                    console.log(id, lastId)
                    if (lastId && this.projects.find((p) => p.id === lastId)) {
                        id = lastId;
                    }
                    this.$router.replace({name: 'overview', params: {projectId: id}});
                }
            });
        },
        lastProject(id) {
            return this.$storage.local('last-project', id);
        },
    },
}
</script>

<style scoped>
.project-name {
    max-width: 10ch;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>

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
/* don't want smaller and bold items in dense lists, e.g. <v-select dense /> */
.v-list--dense .v-list-item .v-list-item__title {
    font-size: inherit;
    font-weight: inherit;
}
</style>
