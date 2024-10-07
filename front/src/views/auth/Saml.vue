<template>
    <v-alert v-if="error" color="red" elevation="2" border="left" colored-border class="mx-auto mt-5" min-width="60%">
        <div v-if="error === 'configuration'">
            <div>Your SAML integration appears to be improperly configured.</div>
            <div>
                Coroot expects to receive the following attributes from your Identity Provider: <b>Email</b>, <b>FirstName</b>, and <b>LastName</b>.
            </div>
        </div>
        <div v-else>Authentication using SAML integration was unsuccessful. Please try again later.</div>
        <div class="mt-2 d-flex" style="gap: 8px">
            <v-btn :to="{ name: 'index' }" color="primary">Refresh</v-btn>
            <v-btn :to="{ name: 'login' }" color="warning">Login as Admin</v-btn>
        </div>
    </v-alert>
</template>

<script>
export default {
    computed: {
        error() {
            return this.$route.query.error;
        },
    },
};
</script>

<style scoped></style>
