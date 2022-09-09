<template>
<div class="mx-auto" style="max-width: 800px">
    <template v-if="projectId">
        <h1 class="text-h5 my-5">
            Status
        </h1>
        <ProjectStatus :projectId="projectId" class="mb-16"/>
    </template>

    <h1 class="text-h5 my-5">
        Project settings
    </h1>
    <ProjectSettings :projectId="projectId" />

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

export default {
    props: {
        projectId: String,
    },

    components: {ProjectSettings, ProjectStatus, ProjectDelete},
}
</script>

<style scoped>
</style>
