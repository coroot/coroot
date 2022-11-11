<template>
<div class="mx-auto">
    <template v-if="projectId">
        <h1 class="text-h5 my-5">
            Status
        </h1>
        <ProjectStatus :projectId="projectId" class="mb-16"/>
    </template>

    <h1 class="text-h5 my-5">
        Project settings
        <a href="https://coroot.com/docs/coroot-community-edition/getting-started/project-configuration" target="_blank">
            <v-icon>mdi-information-outline</v-icon>
        </a>
    </h1>
    <ProjectSettings :projectId="projectId" />

    <template v-if="projectId">
        <h1 class="text-h5 mt-16 mb-5">
            Inspection configs
            <a href="https://coroot.com/docs/coroot-community-edition/inspections/overview" target="_blank">
                <v-icon>mdi-information-outline</v-icon>
            </a>
        </h1>
        <ProjectCheckConfigs :projectId="projectId" />
    </template>

    <template v-if="projectId">
        <h1 class="text-h5 mt-16 mb-5">
            Application categories
            <a href="https://coroot.com/docs/coroot-community-edition/getting-started/project-configuration#application-categories" target="_blank">
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

    <template v-if="projectId">
        <h1 id="integrations" class="text-h5 mt-16 mb-5">
            Integrations
            <a href="https://coroot.com/docs/coroot-community-edition/getting-started/alerting" target="_blank">
                <v-icon>mdi-information-outline</v-icon>
            </a>
        </h1>
        <Integrations />
    </template>

    <template v-if="projectId">
        <h1 class="text-h5 mt-16 mb-5">
            Danger zone
        </h1>
        <ProjectDelete :projectId="projectId" />
    </template>
</div>
</template>

<script>
import ProjectSettings from "@/views/ProjectSettings";
import ProjectStatus from "@/views/ProjectStatus";
import ProjectDelete from "@/views/ProjectDelete";
import ProjectCheckConfigs from "@/views/ProjectCheckConfigs";
import ApplicationCategories from "@/views/ApplicationCategories";
import Integrations from "@/views/Integrations";

export default {
    props: {
        projectId: String,
    },

    components: {ProjectCheckConfigs, ProjectSettings, ProjectStatus, ProjectDelete, ApplicationCategories, Integrations},
}
</script>

<style scoped>
</style>
