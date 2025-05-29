<template>
    <Views :loading="loading" :error="error">
        <Table v-if="nodes && nodes.rows" :header="nodes.header" :rows="nodes.rows" />
        <NoData v-else-if="!loading && !error" />
        <div class="mt-4">
            <AgentInstallation color="primary">Add nodes</AgentInstallation>
        </div>
    </Views>
</template>

<script>
import Views from '@/views/Views.vue';
import AgentInstallation from '@/views/AgentInstallation.vue';
import NoData from '@/components/NoData.vue';
import Table from '@/components/Table.vue';

export default {
    components: { Views, Table, NoData, AgentInstallation },

    data() {
        return {
            nodes: null,
            loading: false,
            error: '',
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    methods: {
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getOverview('nodes', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.nodes = data.nodes;
            });
        },
    },
};
</script>
