<template>
    <div class="wrapper">
        <div class="text-center" style="max-width: 800px">
            <img :src="`${$coroot.base_path}static/icon.svg`" height="80" alt="Coroot Logo" />

            <h1 class="text-h4 mt-5 mb-3">Welcome to Coroot</h1>

            <div v-if="user.readonly">
                <p>Looks like everything is working, but you don’t have access to any projects.</p>
            </div>
            <div v-else>
                <p>Looks like everything is working. Now let's create your first project!</p>
                <v-btn :to="{ name: 'project_new' }" color="primary" large class="mt-5"> Create a project </v-btn>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    props: {
        user: Object,
    },
};
</script>

<style scoped>
.wrapper {
    height: 70vh;
    display: flex;
    align-items: center;
    justify-content: center;
}
</style>
