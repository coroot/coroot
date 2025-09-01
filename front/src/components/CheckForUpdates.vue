<template>
    <v-alert v-if="show" color="green lighten-3" light :height="height" tile dismissible class="ma-0 py-1">
        <template #close>
            <v-btn x-small icon @click="dismiss"><v-icon small>mdi-close</v-icon></v-btn>
        </template>
        <div class="text-center">
            Coroot {{ latestVersion }} is available &#127881;
            <a href="https://github.com/coroot/coroot/releases" target="_blank" class="ml-2 mr-1 link">Changelog</a>
            (<a href="https://docs.coroot.com/" target="_blank" class="link">how to upgrade</a>).
        </div>
    </v-alert>
</template>

<script>
import axios from 'axios';

const key = 'update-alert-dismissed';
export default {
    props: {
        height: Number,
    },

    data() {
        return {
            latestVersion: '',
            ignoredVersion: '',
            ticker: 0,
        };
    },

    mounted() {
        this.ignoredVersion = this.$storage.local(key);
        this.get();
        this.ticker = setInterval(this.get, 3600000);
    },

    beforeDestroy() {
        this.ticker && clearInterval(this.ticker);
    },

    computed: {
        show() {
            return this.latestVersion && this.latestVersion !== this.ignoredVersion;
        },
    },

    watch: {
        show(v) {
            this.$emit('show', v);
        },
    },

    methods: {
        get() {
            const url = this.$coroot.cloud_url + '/ce/version';
            axios
                .get(url, { headers: { 'x-instance-version': this.$coroot.version, 'x-instance-uuid': this.$coroot.instance_uuid } })
                .then((response) => {
                    this.latestVersion = response.data.trim();
                });
        },
        dismiss() {
            this.ignoredVersion = this.latestVersion;
            this.$storage.local(key, this.latestVersion);
        },
    },
};
</script>

<style scoped></style>
