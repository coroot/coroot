<template>
    <div></div>
</template>

<script>
export default {
    created() {
        this.$api.logout(() => {
            this.$router.push({ name: 'index' });
        });
    },
};
</script>

<style scoped></style>
