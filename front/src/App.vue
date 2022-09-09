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
                    <v-list-item v-for="p in projects" :key="p.name" :to="{name: 'overview', params: {projectId: p.id}}">
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

            <span v-if="project" style="position: relative">
                <v-btn v-if="project" icon small :to="{name: 'project_settings', params: {projectId: project.id}}" plain>
                    <v-icon>mdi-cog</v-icon>
                </v-btn>
                <Led v-if="status" :status="status.ok ? 'ok' : 'warning'" style="position: absolute; top: 1px; right: 1px;" />
            </span>
        </v-container>
    </v-app-bar>

    <v-main>
        <v-container>
            <v-alert v-if="status && !status.ok && $route.name !== 'project_settings'" color="red" elevation="2" border="left" class="mt-4" colored-border>
                <div class="d-sm-flex align-center">
                    <template v-if="status.prometheus.status !== 'ok'">
                        <template v-if="status.prometheus.error">
                            <div class="flex-grow-1 mb-3 mb-sm-0">An error has been occurred while querying Prometheus</div>
                            <v-btn outlined :to="{name: 'project_settings'}">Review the configuration</v-btn>
                        </template>
                        <template v-else>
                            <div class="flex-grow-1 mb-3 mb-sm-0">
                                Prometheus cache is {{$moment.duration(status.prometheus.lag, 'ms').format('h [hour] m [minute]', {trim: 'all'})}} behind.
                                Please wait until synchronization is complete.
                            </div>
                            <v-btn outlined @click="refresh">refresh</v-btn>
                        </template>
                    </template>
                    <template v-else-if="status.node_agent.status !== 'ok'">
                        <div class="flex-grow-1 mb-3 mb-sm-0">No metrics found. Looks like you didn't install <b>node-agent</b>.</div>
                        <v-btn outlined :to="{name: 'project_settings'}">Install node-agent</v-btn>
                    </template>
                    <template v-else-if="status.kube_state_metrics && status.kube_state_metrics.status !== 'ok'">
                        <div class="flex-grow-1 mb-3 mb-sm-0">
                            It looks like you use Kubernetes, so Coroot requires <b>kube-state-metrics</b> to combine individual containers into applications.
                        </div>
                        <v-btn outlined :to="{name: 'project_settings'}">Install kube-state-metrics</v-btn>
                    </template>
                </div>
            </v-alert>
            <router-view />
        </v-container>
    </v-main>
</v-app>
</template>

<script>
import TimePicker from "@/components/TimePicker";
import Search from "@/components/Search";
import Led from "@/components/Led";

export default {
    components: {Search, TimePicker, Led},

    data() {
        return {
            projects: [],
            status: null,
            loading: false,
            error: '',
        }
    },

    created() {
        this.getProjects();
        this.$events.watch(this, this.getProjects, 'project-saved', 'project-deleted');
    },

    computed: {
        project() {
            const id = this.$route.params.projectId;
            if (!id) {
                return null;
            }
            return this.projects.find((p) => p.id === id);
        },
    },

    watch: {
        '$route': {
            handler: function() {
                this.getStatus();
            },
            immediate: true,
        },
        '$route.params.projectId': {
            handler: function(id) {
                this.lastProject(id);
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
                    if (lastId && this.projects.find((p) => p.id === lastId)) {
                        id = lastId;
                    }
                    this.$router.replace({name: 'overview', params: {projectId: id}});
                }
            });
        },
        getStatus() {
            this.status = null;
            if (!this.$route.params.projectId) {
                return
            }
            this.$api.getStatus((data, error) => {
                if (error) {
                    return;
                }
                this.status = data;
                this.status.ok = true;
                for (const i in data) {
                    const s = data[i];
                    if (s && s.status && (s.status === 'warning' || s.status === 'critical')) {
                        this.status.ok = false;
                        break;
                    }
                }
            });
        },
        lastProject(id) {
            return this.$storage.local('last-project', id);
        },
        refresh() {
            this.$events.emit('refresh');
            this.getStatus();
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

*::-webkit-scrollbar-track {
    -webkit-box-shadow: inset 0 0 6px rgba(0,0,0,0.3);
    background-color: #F5F5F5;
}
*::-webkit-scrollbar {
    width: 5px;
    height: 5px;
    background-color: #F5F5F5;
}
*::-webkit-scrollbar-thumb {
    background-color: #757575;
}
</style>
