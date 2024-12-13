<template>
    <div>
        <h2 class="text-h6 font-weight-regular">
            Nodes
            <a href="https://docs.coroot.com/costs/overview#nodes" target="_blank">
                <v-icon>mdi-information-outline</v-icon>
            </a>
        </h2>

        <v-data-table
            :items="nodes"
            sort-by="idle_costs"
            sort-desc
            must-sort
            dense
            class="table"
            mobile-breakpoint="0"
            item-key="name"
            :headers="[
                { value: 'name', text: 'Node', align: 'center' },
                { value: 'fake', text: '', align: 'end', sortable: false },
                { value: 'cpu_usage', text: 'CPU', align: 'center', width: '25%' },
                { value: 'memory_usage', text: 'Memory', align: 'center', width: '25%' },
                { value: 'price', text: 'Price', align: 'end' },
                { value: 'idle_costs', text: 'Idle cost', align: 'end', class: 'text-no-wrap' },
                { value: 'cross_az_traffic_costs', text: 'Cross-AZ traffic', align: 'end', class: 'text-no-wrap' },
                { value: 'internet_egress_costs', text: 'Internet egress traffic', align: 'end', class: 'text-no-wrap' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
        >
            <template #item.name="{ item }">
                <router-link :to="{ name: 'overview', params: { view: 'nodes', id: item.name } }" class="name">{{ item.name }}</router-link>
                <div v-if="$vuetify.breakpoint.mdAndUp" class="caption grey--text name">{{ item.description }}</div>
            </template>
            <template #item.fake="{}">
                <div class="caption grey--text">usage:</div>
                <div class="caption grey--text">request:</div>
            </template>
            <template #item.cpu_usage="{ item }">
                <NodeUsageBar v-if="item.cpu_usage_applications" :applications="item.cpu_usage_applications" class="my-1" />
                <NodeUsageBar v-if="item.cpu_request_applications" :applications="item.cpu_request_applications" class="my-1" />
            </template>
            <template #item.memory_usage="{ item }">
                <NodeUsageBar v-if="item.memory_usage_applications" :applications="item.memory_usage_applications" class="my-1" />
                <NodeUsageBar v-if="item.memory_request_applications" :applications="item.memory_request_applications" class="my-1" />
            </template>
            <template #item.price="{ item }">
                ${{ item.price.toFixed(2) }}<span class="caption grey--text">/mo</span>
                <div class="caption grey--text">{{ item.instance_life_cycle }}</div>
            </template>
            <template #item.idle_costs="{ item }"> ${{ item.idle_costs.toFixed(2) }}<span class="caption grey--text">/mo</span> </template>
            <template #item.cross_az_traffic_costs="{ item }">
                ${{ item.cross_az_traffic_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
            </template>
            <template #item.internet_egress_costs="{ item }">
                ${{ item.internet_egress_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
            </template>

            <template #foot>
                <tfoot>
                    <tr v-for="item in [total]">
                        <td class="font-weight-medium">TOTAL</td>
                        <td></td>
                        <td></td>
                        <td></td>
                        <td class="text-right font-weight-medium">${{ item.price.toFixed(2) }}<span class="caption grey--text">/mo</span></td>
                        <td class="text-right font-weight-medium">${{ item.idle_costs.toFixed(2) }}<span class="caption grey--text">/mo</span></td>
                        <td class="text-right font-weight-medium">
                            ${{ item.cross_az_traffic_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                        </td>
                        <td class="text-right font-weight-medium">
                            ${{ item.internet_egress_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                        </td>
                    </tr>
                </tfoot>
            </template>
        </v-data-table>
    </div>
</template>

<script>
import NodeUsageBar from './NodeUsageBar';

export default {
    props: {
        nodes: Array,
    },

    components: { NodeUsageBar },

    computed: {
        total() {
            const res = { price: 0, idle_costs: 0, cross_az_traffic_costs: 0, internet_egress_costs: 0 };
            this.nodes.forEach((n) => {
                res.price += n.price;
                res.idle_costs += n.idle_costs;
                res.cross_az_traffic_costs += n.cross_az_traffic_costs;
                res.internet_egress_costs += n.internet_egress_costs;
            });
            return res;
        },
    },
};
</script>

<style scoped>
.table:deep(table) {
    min-width: 500px;
}
.table:deep(tr:hover) {
    background-color: unset !important;
}
.table:deep(th) {
    white-space: nowrap;
}
.table:deep(th),
.table:deep(td) {
    padding: 4px 8px !important;
}
.table .name {
    display: block;
    max-width: 25vw;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    text-align: left;
}
.table:deep(.v-data-footer) {
    border-top: none;
    flex-wrap: nowrap;
}
</style>
