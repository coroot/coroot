<template>
    <div>
        <v-alert color="info" outlined text>
            Single Sign-On through SAML is available only in Coroot Enterprise.
            <a href="https://coroot.com/contact/" target="_blank" class="font-weight-bold">Contact us</a> for a free trial.
        </v-alert>
        <v-simple-table dense class="params">
            <tbody>
                <tr>
                    <td class="font-weight-medium text-no-wrap">Identity Provider:</td>
                    <td><v-btn color="primary" small disabled>Upload Identity Provider Metadata XML</v-btn></td>
                </tr>
                <tr>
                    <td class="font-weight-medium text-no-wrap">Service Provider Issuer / Identity ID:</td>
                    <td>
                        {{ entity_id }} <v-btn small icon disabled><v-icon small>mdi-content-copy</v-icon></v-btn>
                    </td>
                </tr>
                <tr>
                    <td class="font-weight-medium text-no-wrap">Service Provider ACS URL / Single Sign On URL:</td>
                    <td>
                        {{ asc_url }} <v-btn small icon disabled><v-icon small>mdi-content-copy</v-icon></v-btn>
                    </td>
                </tr>
                <tr>
                    <td class="font-weight-medium text-no-wrap">Attribute mapping:</td>
                    <td>
                        Coroot expects to receive the <b>Email</b>, <b>FirstName</b>, and <b>LastName</b> attributes.
                        <br />
                        Please configure Attribute Mapping on your Identity Provider's side.
                    </td>
                </tr>
                <tr>
                    <td class="font-weight-medium text-no-wrap">Default role:</td>
                    <td>
                        <v-btn small disabled class="pr-1">Viewer <v-icon class="ml-2">mdi-menu-down</v-icon></v-btn>
                    </td>
                </tr>
            </tbody>
        </v-simple-table>
        <v-btn color="primary" disabled class="mt-2">Save and Enable</v-btn>
    </div>
</template>

<script>
export default {
    computed: {
        entity_id() {
            return location.origin;
        },
        asc_url() {
            return location.origin + this.$coroot.base_path + 'api/saml';
        },
    },
};
</script>

<style scoped>
.params:deep(td) {
    padding: 4px 16px !important;
}
</style>
