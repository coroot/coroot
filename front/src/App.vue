<template>
    <v-app>
        <CheckForUpdates v-if="$coroot.check_for_updates" :currentVersion="$coroot.version" :instanceUuid="$coroot.uuid" />

        <v-app-bar app flat dark class="menu">
            <v-container class="py-0 fill-height flex-nowrap">
                <router-link :to="project ? { name: 'overview', query: $utils.contextQuery() } : { name: 'index' }">
                    <img :src="`${$coroot.base_path}static/logo.svg`" height="38" class="logo" alt=":~#" />
                </router-link>

                <div v-if="$route.name !== 'welcome'">
                    <v-menu dark offset-y tile attach=".v-app-bar">
                        <template #activator="{ on, attrs }">
                            <v-btn v-on="on" plain outlined class="ml-3 px-2" height="40">
                                <v-icon small class="mr-2">mdi-hexagon-multiple</v-icon>
                                <template v-if="$vuetify.breakpoint.smAndUp">
                                    <span class="project-name">
                                        <template v-if="project">{{ project.name }}</template>
                                        <template v-else-if="$route.params.projectId">choose a project</template>
                                        <template v-else>new project</template>
                                    </span>
                                    <v-icon small class="ml-2"> mdi-chevron-{{ attrs['aria-expanded'] === 'true' ? 'up' : 'down' }} </v-icon>
                                </template>
                            </v-btn>
                        </template>
                        <v-list dense>
                            <v-list-item v-for="p in projects" :key="p.name" :to="{ name: 'overview', params: { projectId: p.id } }">
                                {{ p.name }}
                            </v-list-item>
                            <v-list-item :to="{ name: 'project_new' }" exact> <v-icon small>mdi-plus</v-icon> new project </v-list-item>
                        </v-list>
                    </v-menu>
                </div>

                <div v-if="$vuetify.breakpoint.mdAndUp && project && $route.name !== 'project_settings'" class="ml-3 flex-grow-1">
                    <Search />
                </div>

                <v-spacer />

                <div v-if="$vuetify.breakpoint.smAndUp" class="ml-3">
                    <v-menu dark offset-y tile attach=".v-app-bar">
                        <template #activator="{ on }">
                            <v-btn v-on="on" plain outlined height="40" class="px-2">
                                <v-icon>mdi-help-circle-outline</v-icon>
                            </v-btn>
                        </template>
                        <v-list dense>
                            <v-list-item href="https://coroot.com/docs/coroot-community-edition" target="_blank">Documentation</v-list-item>
                            <v-list-item href="https://github.com/coroot/coroot" target="_blank">
                                <v-icon small class="mr-1">mdi-github</v-icon>GitHub
                            </v-list-item>
                            <v-list-item
                                href="https://join.slack.com/t/coroot-community/shared_invite/zt-1gsnfo0wj-I~Zvtx5CAAb8vr~r~vecyw"
                                target="_blank"
                            >
                                <v-icon small class="mr-1">mdi-slack</v-icon>Slack chat
                            </v-list-item>
                            <v-list-item href="https://coroot.com/cloud" target="_blank">
                                <v-icon small class="mr-1">mdi-cloud-outline</v-icon>Coroot cloud
                            </v-list-item>
                            <v-divider />
                            <v-list-item href="https://github.com/coroot/coroot/releases" target="_blank">
                                Version: {{ $coroot.version }}
                            </v-list-item>
                        </v-list>
                    </v-menu>
                </div>
                <div v-if="project && $route.name !== 'project_settings'" class="ml-3">
                    <TimePicker :small="$vuetify.breakpoint.xsOnly" />
                </div>

                <v-btn v-if="project" :to="{ name: 'project_settings' }" plain outlined height="40" class="ml-3 px-2">
                    <v-icon>mdi-cog</v-icon>
                    <Led v-if="status" :status="status.status !== 'ok' ? 'warning' : 'ok'" absolute />
                </v-btn>

                <v-menu dark offset-y tile left attach=".v-app-bar">
                    <template #activator="{ on }">
                        <v-btn v-on="on" plain outlined height="40" class="ml-1 px-2">
                            <v-icon>{{ themes[theme] }}</v-icon>
                        </v-btn>
                    </template>
                    <v-list dense>
                        <v-list-item-group v-model="theme">
                            <v-list-item v-for="(icon, name) in themes" @click="setTheme(name)" :value="name">
                                <v-icon small class="mr-1">{{ icon }}</v-icon>
                                {{ name }}
                            </v-list-item>
                        </v-list-item-group>
                    </v-list>
                </v-menu>
            </v-container>
        </v-app-bar>

        <v-main>
            <v-container style="padding-bottom: 128px">
                <v-alert
                    v-if="status && status.status === 'warning' && $route.name !== 'project_settings'"
                    color="red"
                    elevation="2"
                    border="left"
                    class="mt-4"
                    colored-border
                >
                    <div class="d-sm-flex align-center">
                        <template v-if="status.error">
                            {{ status.error }}
                        </template>
                        <template v-else-if="status.prometheus.status !== 'ok'">
                            <div class="flex-grow-1 mb-3 mb-sm-0">{{ status.prometheus.message }}</div>
                            <v-btn
                                v-if="status.prometheus.action === 'configure'"
                                outlined
                                :to="{ name: 'project_settings', params: { tab: 'prometheus' } }"
                            >
                                <template v-if="status.prometheus.error"> Review the configuration </template>
                                <template v-else> Configure </template>
                            </v-btn>
                            <v-btn v-if="status.prometheus.action === 'wait'" outlined @click="refresh">refresh</v-btn>
                        </template>
                        <template v-else-if="status.node_agent.status !== 'ok'">
                            <div class="flex-grow-1 mb-3 mb-sm-0">No metrics found. Looks like you didn't install <b>node-agent</b>.</div>
                            <v-btn outlined :to="{ name: 'project_settings' }">Install node-agent</v-btn>
                        </template>
                        <template v-else-if="status.kube_state_metrics && status.kube_state_metrics.status !== 'ok'">
                            <div class="flex-grow-1 mb-3 mb-sm-0">
                                It looks like you use Kubernetes, so Coroot requires <b>kube-state-metrics</b>
                                to combine individual containers into applications.
                            </div>
                            <v-btn outlined :to="{ name: 'project_settings' }">Install kube-state-metrics</v-btn>
                        </template>
                    </div>
                </v-alert>
                <router-view />
            </v-container>
        </v-main>
    </v-app>
</template>

<script>
import TimePicker from './components/TimePicker.vue';
import Search from './views/Search.vue';
import Led from './components/Led.vue';
import CheckForUpdates from './components/CheckForUpdates.vue';

export default {
    components: { Search, TimePicker, Led, CheckForUpdates },

    data() {
        return {
            projects: [],
            context: this.$api.context,
            theme: this.$storage.local('theme') || 'auto',
        };
    },

    created() {
        this.$events.watch(this, this.getProjects, 'project-saved', 'project-deleted');
        this.getProjects();
        this.setTheme();
    },

    computed: {
        project() {
            const id = this.$route.params.projectId;
            if (!id) {
                return null;
            }
            return this.projects.find((p) => p.id === id);
        },
        status() {
            return this.context.status;
        },
        themes() {
            return {
                light: 'mdi-weather-sunny',
                dark: 'mdi-weather-night',
                auto: 'mdi-theme-light-dark',
            };
        },
    },

    watch: {
        $route(curr, prev) {
            if (curr.query.from !== prev.query.from || curr.query.to !== prev.query.to) {
                this.$events.emit('refresh');
            }
            if (curr.params.projectId !== prev.params.projectId) {
                this.$events.emit('refresh');
                this.lastProject(curr.params.projectId);
            }
            this.getProjects();
        },
    },

    methods: {
        getProjects() {
            this.$api.getProjects((data, error) => {
                if (error) {
                    return;
                }
                this.projects = data || [];
                if (this.$route.name === 'index') {
                    if (!this.projects.length) {
                        this.$router.replace({ name: 'welcome' });
                        return;
                    }
                    let id = this.projects[0].id;
                    const lastId = this.lastProject();
                    if (lastId && this.projects.find((p) => p.id === lastId)) {
                        id = lastId;
                    }
                    this.$router.replace({ name: 'overview', params: { projectId: id } });
                }
            });
        },
        lastProject(id) {
            return this.$storage.local('last-project', id);
        },
        refresh() {
            this.$events.emit('refresh');
        },
        setTheme(theme) {
            const matchMedia = window.matchMedia('(prefers-color-scheme: dark)');
            if (theme) {
                this.theme = theme;
                this.$storage.local('theme', this.theme);
            } else {
                matchMedia.addEventListener('change', (e) => {
                    const theme = this.$storage.local('theme') || 'auto';
                    if (theme === 'auto') {
                        this.$vuetify.theme.dark = e.matches;
                    }
                });
            }
            this.theme = this.$storage.local('theme') || 'auto';
            if (this.theme === 'auto') {
                this.$vuetify.theme.dark = matchMedia.matches;
            } else {
                this.$vuetify.theme.dark = this.theme === 'dark';
            }
        },
    },
};
</script>

<style scoped>
.menu .logo {
    vertical-align: middle;
}
.menu >>> .v-btn {
    min-width: unset !important;
    border-color: rgba(255, 255, 255, 0.2);
}
.menu >>> .v-btn:hover {
    border-color: rgba(255, 255, 255, 1);
}
.project-name {
    max-width: 15ch;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>

<style>
:root {
    --text-light: rgba(0, 0, 0, 0.87);
    --text-light-dimmed: rgba(0, 0, 0, 0.5);
    --text-dark: rgba(255, 255, 255, 0.87);
    --text-dark-dimmed: rgba(255, 255, 255, 0.5);
    --background-light: white;
    --background-light-hi: #eeeeee;
    --background-dark: #121212;
    --background-dark-hi: #616161;
    --brightness-dimmed: 80%;

    --status-unknown: gray;
    --status-ok: green;
    --status-warning: #ff8f00;
    --status-critical: red;
}
.v-application {
    --text-color: var(--text-light);
    --text-color-dimmed: var(--text-light-dimmed);
    --background-color: var(--background-light);
    --background-color-hi: var(--background-light-hi);
    --brightness: 100%;
}
.v-application.theme--dark {
    --text-color: var(--text-dark);
    --text-color-dimmed: var(--text-dark-dimmed);
    --background-color: var(--background-dark);
    --background-color-hi: var(--background-dark-hi);
    --brightness: var(--brightness-dimmed);
}
.v-application.theme--dark .logo {
    filter: brightness(var(--brightness-dimmed));
}
.v-application .v-app-bar,
.v-application .v-app-bar .v-list {
    color: var(--text-dark) !important;
    background-color: var(--background-dark) !important;
}
.v-application {
    color: var(--text-color) !important;
}
.v-application .v-tabs > .v-tabs-bar,
.v-application .v-data-table,
.v-application .v-list,
.v-application .v-card {
    color: var(--text-color) !important;
    background-color: var(--background-color) !important;
}
.v-application .v-icon,
.v-application .v-btn,
.v-application .v-chip {
    filter: brightness(var(--brightness)) !important;
}

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
    -webkit-box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.3);
    background-color: #f5f5f5;
}
*::-webkit-scrollbar {
    width: 5px;
    height: 5px;
    background-color: #f5f5f5;
}
*::-webkit-scrollbar-thumb {
    background-color: #757575;
}
</style>
