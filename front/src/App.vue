<template>
    <v-app>
        <CheckForUpdates v-if="$coroot.check_for_updates" :currentVersion="$coroot.version" :instanceUuid="$coroot.uuid" />

        <v-app-bar app flat dark class="menu">
            <v-container class="py-0 fill-height flex-nowrap">
                <router-link :to="project ? { name: 'overview', query: $utils.contextQuery() } : { name: 'index' }">
                    <img
                        :src="`${$coroot.base_path}static/logo${$coroot.edition === 'Enterprise' ? '-ee' : ''}.svg`"
                        height="38"
                        class="logo"
                        alt=":~#"
                    />
                </router-link>

                <div v-if="user">
                    <v-menu dark offset-y attach=".v-app-bar">
                        <template #activator="{ on, attrs }">
                            <v-btn v-on="on" plain outlined class="ml-3 px-2" height="40">
                                <v-icon small class="mr-2">mdi-hexagon-multiple</v-icon>
                                <template v-if="$vuetify.breakpoint.smAndUp">
                                    <span class="project-name">
                                        <template v-if="project">{{ project.name }}</template>
                                        <template v-else>choose a project</template>
                                    </span>
                                    <v-icon small class="ml-2"> mdi-chevron-{{ attrs['aria-expanded'] === 'true' ? 'up' : 'down' }} </v-icon>
                                </template>
                            </v-btn>
                        </template>
                        <v-list dense>
                            <v-list-item v-for="p in projects" :key="p.name" :to="{ name: 'overview', params: { projectId: p.id } }">
                                {{ p.name }}
                            </v-list-item>
                            <v-list-item v-if="!user.readonly" :to="{ name: 'project_new' }" exact>
                                <v-icon small class="mr-1">mdi-plus</v-icon> new project
                            </v-list-item>
                            <v-list-item v-else-if="!projects.length"> no projects available </v-list-item>
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
                            <v-list-item href="https://docs.coroot.com/" target="_blank">
                                <v-icon small class="mr-1">mdi-book-open-outline</v-icon>Documentation</v-list-item
                            >
                            <v-list-item href="https://github.com/coroot/coroot" target="_blank">
                                <v-icon small class="mr-1">mdi-github</v-icon>GitHub
                            </v-list-item>
                            <v-list-item
                                href="https://join.slack.com/t/coroot-community/shared_invite/zt-1gsnfo0wj-I~Zvtx5CAAb8vr~r~vecyw"
                                target="_blank"
                            >
                                <v-icon small class="mr-1">mdi-slack</v-icon>Slack chat
                            </v-list-item>
                            <v-divider />
                            <v-list-item> Coroot Edition: {{ $coroot.edition }} </v-list-item>
                            <v-list-item href="https://github.com/coroot/coroot/releases" target="_blank">
                                Version: {{ $coroot.version }}
                            </v-list-item>
                        </v-list>
                    </v-menu>
                </div>
                <div v-if="project && $route.name !== 'project_settings'" class="ml-3">
                    <TimePicker :small="$vuetify.breakpoint.xsOnly" />
                </div>

                <v-btn v-if="user" :to="{ name: project ? 'project_settings' : 'project_new' }" plain outlined height="40" class="ml-3 px-2">
                    <v-icon>mdi-cog</v-icon>
                    <Led v-if="status" :status="status.status !== 'ok' ? 'warning' : 'ok'" absolute />
                </v-btn>

                <!-- v-menu.eager is necessary to apply the selected theme -->
                <v-menu dark offset-y left tile eager attach=".v-app-bar">
                    <template #activator="{ on }">
                        <v-btn v-on="on" plain outlined height="40" class="px-2 ml-1">
                            <v-icon>mdi-account</v-icon>
                        </v-btn>
                    </template>
                    <v-list dense>
                        <v-list-item v-if="user">
                            <div>
                                <div>{{ user.name }}</div>
                                <div v-if="user.email" class="caption grey--text">login: {{ user.email }}</div>
                                <div v-if="user.role" class="caption grey--text">role: {{ user.role }}</div>
                            </div>
                        </v-list-item>
                        <v-divider v-if="user" class="my-2" />
                        <v-subheader class="px-4">Theme</v-subheader>
                        <ThemeSelector />
                        <template v-if="user && !user.anonymous">
                            <v-divider class="my-2" />
                            <v-list-item @click="changePassword = true">Change password</v-list-item>
                            <v-list-item :to="{ name: 'logout' }">Sign out</v-list-item>
                        </template>
                    </v-list>
                </v-menu>
            </v-container>
        </v-app-bar>

        <v-main>
            <v-container style="padding-bottom: 128px">
                <v-alert v-if="status && status.status === 'warning'" color="red" elevation="2" border="left" class="mt-4" colored-border>
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
            </v-container>
        </v-main>
    </v-app>
</template>

<script>
import Welcome from '@/views/Welcome.vue';
import TimePicker from './components/TimePicker.vue';
import Search from './views/Search.vue';
import Led from './components/Led.vue';
import CheckForUpdates from './components/CheckForUpdates.vue';
import ThemeSelector from './components/ThemeSelector.vue';
import AgentInstallation from './views/AgentInstallation.vue';
import ChangePassword from './views/auth/ChangePassword.vue';
import './app.css';

export default {
    components: { Welcome, Search, TimePicker, Led, CheckForUpdates, ThemeSelector, AgentInstallation, ChangePassword },

    data() {
        return {
            user: null,
            context: this.$api.context,
            changePassword: false,
        };
    },

    created() {
        this.$events.watch(this, this.getUser, 'projects');
        this.getUser();
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
    },

    watch: {
        $route(curr, prev) {
            this.getUser();
            if (curr.query.from !== prev.query.from || curr.query.to !== prev.query.to || curr.query.incident !== prev.query.incident) {
                this.$events.emit('refresh');
            }
            if (curr.params.projectId !== prev.params.projectId) {
                this.$events.emit('refresh');
                this.lastProject(curr.params.projectId);
            }
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
