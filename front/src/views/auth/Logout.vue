<template>
    <div></div>
</template>

<script>
export default {
    created() {
        this.$api.logout(() => {
            this.$router.push({ name: 'login' });
        });
    },
};
</script>

<style scoped></style>
