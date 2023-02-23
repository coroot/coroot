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
    <NoData v-else-if="!loading" />

    <h1 class="text-h5 my-5">Costs</h1>

    <Table v-if="overview && overview.costs.applications && overview.costs.applications.rows" :header="overview.costs.applications.header" :rows="overview.costs.applications.rows" />

    <h1 class="text-h5 my-5">
        Nodes
    </h1>
    <Table v-if="overview && overview.nodes && overview.nodes.rows" :header="overview.nodes.header" :rows="overview.nodes.rows" />
    <NoData v-else-if="!loading" />
</div>
</template>

<script>
import AppsMap from "@/components/AppsMap";
import Table from "@/components/Table";
import NoData from "@/components/NoData";

export default {
    components: {AppsMap, Table, NoData},

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
