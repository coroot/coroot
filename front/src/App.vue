<template>
    <v-app>
        <CheckForUpdates v-if="$coroot.check_for_updates" :currentVersion="$coroot.version" :instanceUuid="$coroot.instance_uuid" />

        <v-navigation-drawer v-if="menu" permanent app dark :mini-variant="menuCollapsed" width="188" stateless>
            <template #prepend>
                <div class="mx-2 my-3">
                    <router-link :to="project ? { name: 'overview', query: $utils.contextQuery() } : { name: 'index' }">
                        <img :src="`${$coroot.base_path}static/${logo}`" height="38" class="logo" alt=":~#" />
                    </router-link>
                </div>
                <v-list v-if="project" dense class="pa-0">
                    <v-list-item @click="search = true">
                        <v-list-item-icon class="mr-3">
                            <v-icon dark>mdi-magnify</v-icon>
                        </v-list-item-icon>
                        <v-list-item-content class="text-no-wrap">Go to...</v-list-item-content>
                        <v-list-item-action class="my-0">{{ mac ? '⌘' : 'ctrl' }}+k</v-list-item-action>
                    </v-list-item>
                    <v-divider class="ma-3" style="border-color: var(--border-dark)"></v-divider>
                </v-list>
            </template>

            <v-list v-if="project" dense class="ma-0 pa-0">
                <v-list-item
                    v-for="(v, id) in views"
                    :to="{
                        name: 'overview',
                        params: { view: id, id: undefined, report: undefined },
                        query: id === 'incidents' ? { ...$utils.contextQuery(), incident: undefined } : $utils.contextQuery(),
                    }"
                    :class="{ 'v-list-item--active': id === view }"
                >
                    <v-list-item-icon class="mr-3">
                        <span v-if="id === 'incidents' && menuCollapsed && incidentsCount">
                            <v-badge color="red" dot offset-y="6">
                                <v-icon dark>{{ v.icon }}</v-icon>
                            </v-badge>
                        </span>
                        <v-icon v-else dark>{{ v.icon }}</v-icon>
                    </v-list-item-icon>
                    <v-list-item-content>
                        <span v-if="id === 'incidents' && incidentsCount">
                            <v-badge color="red" :content="incidentsCount" offset-y="12" offset-x="-3" class="badge">
                                {{ v.name }}
                            </v-badge>
                        </span>
                        <template v-else>{{ v.name }}</template>
                    </v-list-item-content>
                </v-list-item>
            </v-list>

            <template #append>
                <v-list dense class="ma-0 pa-0">
                    <v-divider class="ma-3" style="border-color: var(--border-dark)"></v-divider>
                    <v-menu v-if="user" dark right offset-x tile>
                        <template #activator="{ on }">
                            <v-list-item v-on="on">
                                <v-list-item-icon class="mr-3">
                                    <v-icon dark>mdi-hexagon-multiple</v-icon>
                                </v-list-item-icon>
                                <v-list-item-content class="pa-0">
                                    <v-list-item-subtitle class="mb-0">Project</v-list-item-subtitle>
                                    <v-list-item-title style="line-height: inherit">
                                        <template v-if="project">{{ project.name }}</template>
                                        <template v-else>choose a project</template>
                                    </v-list-item-title>
                                </v-list-item-content>
                            </v-list-item>
                        </template>
                        <v-list dense class="pa-0">
                            <v-list-item v-for="p in projects" :key="p.name" :to="{ name: 'overview', params: { projectId: p.id } }">
                                {{ p.name }}
                            </v-list-item>
                            <v-list-item v-if="!user.readonly" :to="{ name: 'project_new' }" exact>
                                <v-icon small class="mr-1">mdi-plus</v-icon> new project
                            </v-list-item>
                            <v-list-item v-else-if="!projects.length"> no projects available </v-list-item>
                        </v-list>
                    </v-menu>

                    <v-list-item :to="{ name: project ? 'project_settings' : 'project_new' }">
                        <v-list-item-icon class="mr-3">
                            <v-icon dark>mdi-cog</v-icon>
                        </v-list-item-icon>
                        <v-list-item-content> Settings </v-list-item-content>
                    </v-list-item>

                    <!-- v-menu.eager is necessary to apply the selected theme -->
                    <v-menu v-if="user" dark right offset-x tile eager>
                        <template #activator="{ on }">
                            <v-list-item v-on="on">
                                <v-list-item-icon class="mr-3">
                                    <v-icon dark>mdi-account</v-icon>
                                </v-list-item-icon>
                                <v-list-item-content>
                                    <v-list-item-title>{{ user.name }}</v-list-item-title>
                                </v-list-item-content>
                            </v-list-item>
                        </template>
                        <v-list dense class="pa-0">
                            <v-list-item v-if="user">
                                <div class="py-2">
                                    <div>{{ user.name }}</div>
                                    <div v-if="user.email" class="caption grey--text">login: {{ user.email }}</div>
                                    <div v-if="user.role" class="caption grey--text">role: {{ user.role }}</div>
                                </div>
                            </v-list-item>
                            <v-divider v-if="user" class="ma-2" />
                            <v-subheader class="px-4">Theme</v-subheader>
                            <ThemeSelector />
                            <template v-if="user && !user.anonymous">
                                <v-divider class="my-2" />
                                <v-list-item @click="changePassword = true">Change password</v-list-item>
                                <v-list-item :to="{ name: 'logout' }">Sign out</v-list-item>
                            </template>
                        </v-list>
                    </v-menu>

                    <v-menu dark right offset-x tile>
                        <template #activator="{ on }">
                            <v-list-item v-on="on">
                                <v-list-item-icon class="mr-3">
                                    <v-icon dark>mdi-help-circle-outline</v-icon>
                                </v-list-item-icon>
                                <v-list-item-content>Help</v-list-item-content>
                            </v-list-item>
                        </template>
                        <v-list dense class="pa-0">
                            <v-list-item href="https://docs.coroot.com/" target="_blank">
                                <v-icon small class="mr-1">mdi-book-open-outline</v-icon>Documentation</v-list-item
                            >
                            <v-list-item href="https://github.com/coroot/coroot" target="_blank">
                                <v-icon small class="mr-1">mdi-github</v-icon>GitHub
                            </v-list-item>
                            <v-list-item href="https://coroot.com/join-slack-community/" target="_blank">
                                <v-icon small class="mr-1">mdi-slack</v-icon>Slack chat
                            </v-list-item>
                            <v-divider />
                            <v-list-item> Coroot Edition: {{ $coroot.edition }} </v-list-item>
                            <v-list-item href="https://github.com/coroot/coroot/releases" target="_blank">
                                Version: {{ $coroot.version }}
                            </v-list-item>
                        </v-list>
                    </v-menu>

                    <v-list-item @click="toggleMenu">
                        <v-list-item-icon class="mr-3">
                            <v-icon v-if="menuCollapsed" dark>mdi-chevron-right</v-icon>
                            <v-icon v-else dark>mdi-chevron-left</v-icon>
                        </v-list-item-icon>
                        <v-list-item-content> Collapse </v-list-item-content>
                    </v-list-item>
                </v-list>
            </template>
        </v-navigation-drawer>

        <v-main>
            <v-container fluid class="py-5 px-5">
                <v-alert
                    v-if="status && status.status === 'warning' && $route.name !== 'project_settings'"
                    color="red"
                    elevation="2"
                    border="left"
                    class="mb-4"
                    colored-border
                >
                    <div class="d-sm-flex align-center" style="gap: 8px">
                        <template v-if="status.error">
                            {{ status.error }}
                        </template>
                        <template v-else-if="status.prometheus.status !== 'ok'">
                            <div class="flex-grow-1 mb-3 mb-sm-0">
                                {{ status.prometheus.message }}
                                <div v-if="status.prometheus.error" class="mt-1" style="font-size: 14px">
                                    {{ status.prometheus.error }}
                                </div>
                            </div>
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
                            <div class="flex-grow-1 mb-3 mb-sm-0">
                                No metrics found. If you just installed Coroot and node-agent, please wait a couple minutes for it to collect data.
                                <br />
                                If you haven't installed node-agent, please do so now.
                            </div>
                            <AgentInstallation outlined>Install node-agent</AgentInstallation>
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

                <Welcome v-if="$route.name === 'index' && user && !projects.length" :user="user" />

                <router-view v-else />

                <ChangePassword v-if="user" v-model="changePassword" />

                <CloudPromoDialog v-if="!ee && user" />

                <Search v-if="search" v-model="search" />
            </v-container>
        </v-main>
    </v-app>
</template>

<script>
import Welcome from '@/views/Welcome.vue';
import Search from './views/Search.vue';
import CheckForUpdates from './components/CheckForUpdates.vue';
import ThemeSelector from './components/ThemeSelector.vue';
import AgentInstallation from './views/AgentInstallation.vue';
import ChangePassword from './views/auth/ChangePassword.vue';
import CloudPromoDialog from './components/CloudPromoDialog.vue';
import { views } from '@/views/Views.vue';
import './app.css';

export default {
    components: { Welcome, Search, CheckForUpdates, ThemeSelector, AgentInstallation, ChangePassword, CloudPromoDialog },

    data() {
        let menuCollapsed = this.$storage.local('menu-collapsed');
        if (menuCollapsed === undefined) {
            menuCollapsed = this.$vuetify.breakpoint.xsOnly;
        }
        return {
            user: null,
            context: this.$api.context,
            changePassword: false,
            menuCollapsed: menuCollapsed,
            search: false,
            sss: '',
        };
    },

    mounted() {
        this.$events.watch(this, this.getUser, 'projects');
        this.getUser();
        window.addEventListener('keydown', this.searchListener);
    },

    beforeDestroy() {
        window.removeEventListener('keydown', this.searchListener);
    },

    computed: {
        projects() {
            if (!this.user) {
                return [];
            }
            return this.user.projects || [];
        },
        project() {
            const id = this.$route.params.projectId;
            if (!id) {
                return null;
            }
            return this.projects.find((p) => p.id === id);
        },
        status() {
            return this.project ? this.context.status : null;
        },
        view() {
            return this.$route.params.view;
        },
        views() {
            return views;
        },
        logo() {
            if (this.menuCollapsed) {
                return 'icon.svg';
            }
            if (this.ee) {
                return 'logo-ee.svg';
            }
            return 'logo.svg';
        },
        ee() {
            return this.$coroot.edition === 'Enterprise';
        },
        menu() {
            return !this.$route.meta.anonymous;
        },
        mac() {
            return /Mac|iPod|iPhone|iPad/.test(navigator.platform);
        },
        incidentsCount() {
            return Object.values(this.context.incidents).reduce((acc, current) => {
                return acc + current;
            }, 0);
        },
    },

    watch: {
        $route: {
            handler(curr, prev) {
                this.getUser();
                if (curr.name === 'overview' && !this.views[curr.params.view]) {
                    this.$router.replace({ params: { view: 'applications' } }).catch((err) => err);
                    return;
                }
                if (!prev) {
                    return;
                }
                if (curr.query.from !== prev.query.from || curr.query.to !== prev.query.to || curr.query.incident !== prev.query.incident) {
                    this.$events.emit('refresh');
                }
            },
            immediate: true,
        },
        '$route.params.projectId'(v) {
            this.$events.emit('refresh');
            this.lastProject(v);
        },
    },

    methods: {
        getUser() {
            if (this.$route.meta.anonymous) {
                return;
            }
            this.$api.user(null, (data, error) => {
                if (error) {
                    this.user = null;
                    return;
                }
                this.user = data;
                if (this.$route.name === 'index' && this.projects.length) {
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
        toggleMenu() {
            this.menuCollapsed = !this.menuCollapsed;
            this.$storage.local('menu-collapsed', this.menuCollapsed);
        },
        searchListener(e) {
            if (this.project && (e.metaKey || e.ctrlKey) && e.key === 'k') {
                e.preventDefault();
                this.search = true;
            }
        },
    },
};
</script>

<style scoped>
.badge:deep(.v-badge__badge) {
    height: 16px;
    min-width: 16px;
    padding: 2px 4px;
}
</style>
