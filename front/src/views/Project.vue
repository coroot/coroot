<template>
    <div class="mx-auto">
        <h1 class="text-h5 my-5">Configuration</h1>

        <v-tabs height="40" show-arrows slider-size="2">
            <v-tab v-for="t in tabs" :key="t.id" :to="{ params: { tab: t.id } }" :disabled="t.id && !projectId" exact>
                {{ t.name }}
            </v-tab>
        </v-tabs>

        <template v-if="!tab">
            <h2 class="text-h5 my-5">Project name</h2>
            <ProjectSettings :projectId="projectId" />

            <template v-if="projectId">
                <h2 class="text-h5 mt-10 mb-5">Status</h2>
                <ProjectStatus :projectId="projectId" />

                <h2 class="text-h5 mt-10 mb-5">Danger zone</h2>
                <ProjectDelete :projectId="projectId" />
            </template>
        </template>

        <template v-if="tab === 'prometheus'">
            <h1 class="text-h5 my-5">
                Prometheus integration
                <a href="https://coroot.com/docs/coroot-community-edition/getting-started/project-configuration" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <IntegrationPrometheus :projectId="projectId" />
        </template>

        <template v-if="tab === 'clickhouse'">
            <h1 class="text-h5 my-5">ClickHouse integration</h1>
            <p>
                Coroot stores
                <a href="https://coroot.com/docs/coroot-community-edition/logs" target="_blank">logs</a>,
                <a href="https://coroot.com/docs/coroot-community-edition/tracing" target="_blank">traces</a>, and
                <a href="https://coroot.com/docs/coroot-community-edition/profiling" target="_blank">profiles</a> in the ClickHouse database.
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
                <a href="https://coroot.com/docs/coroot-community-edition/inspections/overview" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <ProjectCheckConfigs :projectId="projectId" />
        </template>

        <template v-if="tab === 'applications'">
            <h2 class="text-h5 my-5" id="categories">
                Application categories
                <a
                    href="https://coroot.com/docs/coroot-community-edition/getting-started/project-configuration#application-categories"
                    target="_blank"
                >
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h2>
            <p>
                You can organize your applications into groups by defining
                <a href="https://en.wikipedia.org/wiki/Glob_(programming)" target="_blank">glob patterns</a>
                in the <var>&lt;namespace&gt;/&lt;application_name&gt;</var> format.
            </p>
            <ApplicationCategories />

            <h2 class="text-h5 mt-10 mb-5" id="custom-applications">Custom applications</h2>

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
                <a href="https://coroot.com/docs/coroot-community-edition/getting-started/alerting" target="_blank">
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <Integrations />
        </template>
    </div>
</template>

<script>
import ProjectSettings from './ProjectSettings.vue';
import ProjectStatus from './ProjectStatus.vue';
import ProjectDelete from './ProjectDelete.vue';
import ProjectCheckConfigs from './ProjectCheckConfigs.vue';
import ApplicationCategories from './ApplicationCategories.vue';
import Integrations from './Integrations.vue';
import IntegrationPrometheus from './IntegrationPrometheus.vue';
import IntegrationClickhouse from './IntegrationClickhouse.vue';
import IntegrationAWS from './IntegrationAWS.vue';
import CustomApplications from '@/views/CustomApplications.vue';

const tabs = [
    { id: undefined, name: 'General' },
    { id: 'prometheus', name: 'Prometheus' },
    { id: 'clickhouse', name: 'Clickhouse' },
    { id: 'aws', name: 'AWS' },
    { id: 'inspections', name: 'Inspections' },
    { id: 'applications', name: 'Applications' },
    { id: 'notifications', name: 'Notifications' },
];

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
        ProjectCheckConfigs,
        ProjectSettings,
        ProjectStatus,
        ProjectDelete,
        ApplicationCategories,
        Integrations,
    },

    mounted() {
        if (!this.tabs.find((t) => t.id === this.tab)) {
            this.$router.replace({ params: { tab: undefined } });
        }
    },

    computed: {
        tabs() {
            return tabs;
        },
    },
};
</script>

<style scoped></style>
