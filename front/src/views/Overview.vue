<template>
<div>
    <h1 class="text-h5 my-5">
        Applications
        <v-progress-linear v-if="loading" indeterminate color="green" />
    </h1>

    <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
        {{error}}
    </v-alert>

    <AppsMap v-if="overview && overview.applications" :applications="overview.applications" />
    <div v-else-if="overview" class="pa-3 text-center grey--text">
        No data is available yet
    </div>

    <h1 class="text-h5 my-5">
        Nodes
    </h1>

    <Table v-if="overview && overview.nodes && overview.nodes.rows" :header="overview.nodes.header" :rows="overview.nodes.rows" />
    <div v-else-if="overview" class="pa-3 text-center grey--text">
        No data is available yet
    </div>
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
        this.$events.watch(this, this.get, 'refresh');
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
