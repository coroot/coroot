<template>
<div class="mx-auto" style="max-width: 800px">
    <h1 v-if="projectId" class="text-h5 my-5">
        Status
    </h1>
    <ProjectStatus v-if="projectId" :projectId="projectId" class="mb-16"/>

    <h1 class="text-h5 my-5">
        Project settings
    </h1>
    <ProjectSettings :projectId="projectId" />
</div>
</template>

<script>
import ProjectSettings from "@/components/ProjectSettings";
import ProjectStatus from "@/components/ProjectStatus";

export default {
    props: {
        projectId: String,
    },

    components: {ProjectSettings, ProjectStatus},
}
</script>

<style scoped>
</style>
