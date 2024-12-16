<template>
    <div class="mx-auto">
        <h1 class="text-h5 my-5">Configuration</h1>

        <v-tabs height="40" show-arrows slider-size="2">
            <v-tab v-for="t in tabs" :key="t.id" :to="{ params: { tab: t.id } }" :disabled="t.disabled" exact>
                {{ t.name }}
            </v-tab>
        </v-tabs>

        <template v-if="!tab">
            <h2 class="text-h5 my-5">Project name</h2>
            <ProjectSettings :projectId="projectId" />

            <template v-if="projectId">
                <h2 class="text-h5 mt-10 mb-5">Status</h2>
                <ProjectStatus :projectId="projectId" />

                <h2 class="text-h5 mt-10 mb-5">API keys</h2>
                <p>The API keys below authorize Coroot's agents and other applications to write telemetry data for this project.</p>
                <ProjectApiKeys />

                <h2 class="text-h5 mt-10 mb-5">Danger zone</h2>
                <ProjectDelete :projectId="projectId" />
            </template>
        </template>

        <template v-if="tab === 'prometheus'">
            <h1 class="text-h5 my-5">
                Prometheus integration
                <a href="https://docs.coroot.com/configuration/prometheus" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <IntegrationPrometheus />
        </template>

        <template v-if="tab === 'clickhouse'">
            <h1 class="text-h5 my-5">
                ClickHouse integration
                <a href="https://docs.coroot.com/configuration/clickhouse" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <p>
                Coroot stores
                <a href="https://docs.coroot.com/logs" target="_blank">logs</a>, <a href="https://docs.coroot.com/tracing" target="_blank">traces</a>,
                and <a href="https://docs.coroot.com/profiling" target="_blank">profiles</a> in the ClickHouse database.
            </p>
            <IntegrationClickhouse />
        </template>

        <template v-if="tab === 'aws'">
            <h1 class="text-h5 my-5">AWS integration</h1>
            <IntegrationAWS />
        </template>

        <template v-if="tab === 'inspections'">
            <h1 class="text-h5 my-5">
                Inspection configs
                <a href="https://docs.coroot.com/inspections" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <Inspections />
        </template>

        <template v-if="tab === 'applications'">
            <h2 class="text-h5 my-5" id="categories">
                Application categories
                <a href="https://docs.coroot.com/configuration/application-categories" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h2>
            <p>
                You can organize your applications into groups by defining
                <a href="https://en.wikipedia.org/wiki/Glob_(programming)" target="_blank">glob patterns</a>
                in the <var>&lt;namespace&gt;/&lt;application_name&gt;</var> format.
            </p>
            <ApplicationCategories />

            <h2 class="text-h5 mt-10 mb-5" id="custom-applications">
                Custom applications
                <a href="https://docs.coroot.com/configuration/custom-applications" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h2>

            <p>Coroot groups individual containers into applications using the following approach:</p>

            <ul>
                <li><b>Kubernetes metadata</b>: Pods are grouped into Deployments, StatefulSets, etc.</li>
                <li>
                    <b>Non-Kubernetes containers</b>: Containers such as Docker containers or Systemd units are grouped into applications by their
                    names. For example, Systemd services named <var>mysql</var> on different hosts are grouped into a single application called
                    <var>mysql</var>.
                </li>
            </ul>

            <p class="my-5">
                This default approach works well in most cases. However, since no one knows your system better than you do, Coroot allows you to
                manually adjust application groupings to better fit your specific needs. You can match desired application instances by defining
                <a href="https://en.wikipedia.org/wiki/Glob_(programming)" target="_blank">glob patterns</a>
                for <var>instance_name</var>. Note that this is not applicable to Kubernetes applications.
            </p>

            <CustomApplications />
        </template>

        <template v-if="tab === 'notifications'">
            <h1 class="text-h5 my-5">
                Notification integrations
                <a href="https://docs.coroot.com/alerting/slo-monitoring" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <Integrations />
        </template>

        <template v-if="tab === 'organization'">
            <h1 class="text-h5 my-5">
                Users
                <a href="https://docs.coroot.com/configuration/authentication" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <Users />
            <h1 class="text-h5 mt-10 mb-5">
                Role-Based Access Control (RBAC)
                <a href="https://docs.coroot.com/configuration/rbac" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <RBAC />
            <h1 class="text-h5 mt-10 mb-5">
                Single Sign-On (SAML)
                <a href="https://docs.coroot.com/configuration/authentication/#single-sign-on-sso" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <SSO />
        </template>
    </div>
</template>

<script>
import ProjectSettings from './ProjectSettings.vue';
import ProjectStatus from './ProjectStatus.vue';
import ProjectApiKeys from './ProjectApiKeys.vue';
import ProjectDelete from './ProjectDelete.vue';
import Inspections from './Inspections.vue';
import ApplicationCategories from './ApplicationCategories.vue';
import Integrations from './Integrations.vue';
import IntegrationPrometheus from './IntegrationPrometheus.vue';
import IntegrationClickhouse from './IntegrationClickhouse.vue';
import IntegrationAWS from './IntegrationAWS.vue';
import CustomApplications from './CustomApplications.vue';
import Users from './Users.vue';
import RBAC from './RBAC.vue';
import SSO from './SSO.vue';

export default {
    props: {
        projectId: String,
        tab: String,
    },

    components: {
        CustomApplications,
        IntegrationPrometheus,
        IntegrationClickhouse,
        IntegrationAWS,
        Inspections,
        ProjectSettings,
        ProjectStatus,
        ProjectApiKeys,
        ProjectDelete,
        ApplicationCategories,
        Integrations,
        Users,
        RBAC,
        SSO,
    },

    mounted() {
        if (!this.tabs.find((t) => t.id === this.tab)) {
            this.$router.replace({ params: { tab: undefined } });
        }
    },

    computed: {
        tabs() {
            const disabled = !this.projectId;
            return [
                { id: undefined, name: 'General' },
                { id: 'prometheus', name: 'Prometheus', disabled },
                { id: 'clickhouse', name: 'Clickhouse', disabled },
                { id: 'aws', name: 'AWS', disabled },
                { id: 'inspections', name: 'Inspections', disabled },
                { id: 'applications', name: 'Applications', disabled },
                { id: 'notifications', name: 'Notifications', disabled },
                { id: 'organization', name: 'Organization' },
            ];
        },
    },
};
</script>

<style scoped></style>
