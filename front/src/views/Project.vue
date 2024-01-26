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
            <h1 class="text-h5 my-5">Clickhouse integration</h1>
            <p>
                Coroot can display OpenTelemetry traces and logs stored in a ClickHouse database (<a
                    href="https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/clickhouseexporter"
                    target="_blank"
                    >clickhouse-exporter</a
                >).
            </p>
            <IntegrationClickhouse />
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

        <template v-if="tab === 'categories'">
            <h1 class="text-h5 my-5">
                Application categories
                <a
                    href="https://coroot.com/docs/coroot-community-edition/getting-started/project-configuration#application-categories"
                    target="_blank"
                >
                    <v-icon>mdi-information-outline</v-icon>
                </a>
            </h1>
            <p>
                You can organize your applications into groups by defining
                <a href="https://en.wikipedia.org/wiki/Glob_(programming)" target="_blank">glob patterns</a>
                in the <var>&lt;namespace&gt;/&lt;application_name&gt;</var> format.
            </p>
            <ApplicationCategories />
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

const tabs = [
    { id: undefined, name: 'General' },
    { id: 'prometheus', name: 'Prometheus' },
    { id: 'clickhouse', name: 'Clickhouse' },
    { id: 'inspections', name: 'Inspections' },
    { id: 'categories', name: 'Categories' },
    { id: 'notifications', name: 'Notifications' },
];

export default {
    props: {
        projectId: String,
        tab: String,
    },

    components: {
        IntegrationPrometheus,
        IntegrationClickhouse,
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
