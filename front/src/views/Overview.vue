<template>
<div>
    <h1 class="text-h5 my-5">
        Applications
        <v-progress-linear v-if="loading" indeterminate color="green" />
    </h1>

    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
        {{error}}
    </v-alert>

    <AppsMap v-if="overview" :applications="overview.applications" />

    <h1 class="text-h5 my-5">
        Nodes
    </h1>

    <Table v-if="overview" :header="overview.nodes.header" :rows="overview.nodes.rows" />
</div>
</template>

<script>
import AppsMap from "@/components/AppsMap";
import Table from "@/components/Table";

export default {
    components: {AppsMap, Table},

    data() {
        return {
            overview: null,
            loading: false,
            error: '',
        }
    },

    mounted() {
        this.get();
        this.$api.contextWatch(this, this.get);
    },

    methods: {
        get() {
            this.loading = true;
            this.$api.getOverview((data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.overview = data;
            });
        }
    },
};
</script>
