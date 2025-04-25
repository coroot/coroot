<template>
    <img
        v-if="icon"
        :src="`${$coroot.base_path}static/img/tech-icons/${icon}.svg`"
        :alt="icon + ' icon'"
        :title="icon"
        onerror="this.style.display='none'"
        :height="size"
        :width="size"
    />
</template>

<script>
export default {
    props: {
        icon: String,
        size: { type: Number, default: 16 },
    },
};
</script>

<style scoped>
img {
    opacity: 80%;
}
</style>
