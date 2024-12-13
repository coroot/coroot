<template>
    <div>
        <h2 class="text-h6 font-weight-regular d-md-flex align-center mb-3">
            <v-btn v-if="category !== null" icon @click="back"><v-icon>mdi-arrow-left</v-icon></v-btn>
            Applications
            <a href="https://docs.coroot.com/costs/overview#applications" target="_blank" class="ml-1">
                <v-icon>mdi-information-outline</v-icon>
            </a>
            <v-chip v-if="category" @click:close="category = ''" label close color="primary" class="ml-3"> category: {{ category }} </v-chip>
            <v-chip v-if="application" @click:close="application = null" label close color="primary" class="ml-3">
                application: {{ $utils.appId(application.id).name }}
            </v-chip>
            <v-spacer />
            <span v-if="category !== null && application === null && $vuetify.breakpoint.mdAndUp" style="max-width: 50%">
                <v-text-field
                    v-model="search"
                    dense
                    hide-details
                    clearable
                    prepend-inner-icon="mdi-magnify"
                    label="Search"
                    single-line
                    outlined
                    class="search"
                />
            </span>
        </h2>

        <v-data-table
            v-if="category === null"
            sort-by="usage_costs"
            sort-desc
            must-sort
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="10"
            :items="categories"
            item-key="name"
            :headers="[
                { value: 'name', text: 'Category', align: 'center' },
                { value: 'usage_costs', text: 'Usage costs', align: 'end' },
                { value: 'allocation_costs', text: 'Allocation costs', align: 'end' },
                { value: 'over_provisioning_costs', text: 'Overprovisioning costs', align: 'end' },
                { value: 'cross_az_traffic_costs', text: 'Cross-AZ traffic', align: 'end' },
                { value: 'internet_egress_costs', text: 'Internet egress traffic', align: 'end' },
            ]"
            :footer-props="{ itemsPerPageOptions: [5, 10, 20, 50, 100, -1] }"
        >
            <template #item.name="{ item }">
                <a class="name" @click="category = item.name">{{ item.name }}</a>
            </template>
            <template #item.usage_costs="{ item }"> ${{ item.usage_costs.toFixed(2) }}<span class="caption grey--text">/mo</span> </template>
            <template #item.allocation_costs="{ item }">
                <template v-if="item.allocation_costs > 0">
                    ${{ item.allocation_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                </template>
                <template v-else>—</template>
            </template>
            <template #item.over_provisioning_costs="{ item }">
                <template v-if="item.over_provisioning_costs > 0">
                    ${{ item.over_provisioning_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                </template>
                <template v-else>—</template>
            </template>
            <template #item.cross_az_traffic_costs="{ item }">
                <template v-if="item.cross_az_traffic_costs > 0">
                    ${{ item.cross_az_traffic_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                </template>
                <template v-else>—</template>
            </template>
            <template #item.internet_egress_costs="{ item }">
                <template v-if="item.internet_egress_costs > 0">
                    ${{ item.internet_egress_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                </template>
                <template v-else>—</template>
            </template>
            <template #foot>
                <tfoot>
                    <tr v-for="item in [categoriesTotal]">
                        <td class="font-weight-medium">
                            <a @click="category = ''">TOTAL</a>
                        </td>
                        <td class="font-weight-medium text-right">${{ item.usage_costs.toFixed(2) }}<span class="caption grey--text">/mo</span></td>
                        <td class="font-weight-medium text-right">
                            <template v-if="item.allocation_costs > 0">
                                ${{ item.allocation_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                            </template>
                            <template v-else>—</template>
                        </td>
                        <td class="font-weight-medium text-right">
                            <template v-if="item.over_provisioning_costs > 0">
                                ${{ item.over_provisioning_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                            </template>
                            <template v-else>—</template>
                        </td>
                        <td class="font-weight-medium text-right">
                            <template v-if="item.cross_az_traffic_costs > 0">
                                ${{ item.cross_az_traffic_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                            </template>
                            <template v-else>—</template>
                        </td>
                        <td class="font-weight-medium text-right">
                            <template v-if="item.internet_egress_costs > 0">
                                ${{ item.internet_egress_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                            </template>
                            <template v-else>—</template>
                        </td>
                    </tr>
                </tfoot>
            </template>
        </v-data-table>

        <v-data-table
            v-else-if="!application"
            sort-by="usage_costs"
            sort-desc
            must-sort
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="20"
            :items="filteredApplications"
            item-key="id"
            :headers="[
                { value: 'name', text: 'Application', align: 'center' },
                { value: 'usage_costs', text: 'Usage costs', align: 'end', filterable: false },
                { value: 'allocation_costs', text: 'Allocation costs', align: 'end', filterable: false },
                { value: 'over_provisioning_costs', text: 'Overprovisioning costs', align: 'end', filterable: false },
                { value: 'cross_az_traffic_costs', text: 'Cross-AZ traffic', align: 'end', filterable: false },
                { value: 'internet_egress_costs', text: 'Internet egress traffic', align: 'end', filterable: false },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
            :search="search"
            :custom-filter="searchApplication"
        >
            <template #item.name="{ item }">
                <a class="name" @click="application = item">{{ $utils.appId(item.id).name }}</a>
            </template>
            <template #item.usage_costs="{ item }"> ${{ item.usage_costs.toFixed(2) }}<span class="caption grey--text">/mo</span> </template>
            <template #item.allocation_costs="{ item }">
                <template v-if="item.allocation_costs > 0">
                    ${{ item.allocation_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                </template>
                <template v-else>—</template>
            </template>
            <template #item.over_provisioning_costs="{ item }">
                <template v-if="item.over_provisioning_costs > 0">
                    ${{ item.over_provisioning_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                </template>
                <template v-else>—</template>
            </template>
            <template #item.cross_az_traffic_costs="{ item }">
                <template v-if="item.cross_az_traffic_costs > 0">
                    ${{ item.cross_az_traffic_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                </template>
                <template v-else>—</template>
            </template>
            <template #item.internet_egress_costs="{ item }">
                <template v-if="item.internet_egress_costs > 0">
                    ${{ item.internet_egress_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                </template>
                <template v-else>—</template>
            </template>
        </v-data-table>

        <div v-else>
            <div class="text-right">
                <router-link :to="{ name: 'overview', params: { view: 'applications', id: application.id } }">
                    {{ $utils.appId(application.id).name }}
                    <v-icon small>mdi-open-in-new</v-icon>
                </router-link>
            </div>
            <v-simple-table dense class="table mt-5">
                <thead>
                    <tr>
                        <th>Application</th>
                        <th class="text-right">CPU request</th>
                        <th class="text-right">Memory request</th>
                        <th class="text-right">Allocation costs</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="c in application.components" :key="c.name">
                        <td class="py-1">
                            {{ c.name }}
                            <div v-if="c.kind" class="caption grey--text">{{ c.kind }}</div>
                        </td>
                        <td class="text-right">
                            {{ c.cpu_request ? c.cpu_request : '—' }}
                            <div v-if="c.cpu_request_recommended" class="caption green--text">recommended: {{ c.cpu_request_recommended }}</div>
                        </td>
                        <td class="text-right">
                            {{ c.memory_request ? c.memory_request : '—' }}
                            <div v-if="c.memory_request_recommended" class="caption green--text">recommended: {{ c.memory_request_recommended }}</div>
                        </td>
                        <td class="text-right">
                            ${{ c.allocation_costs.toFixed(2) }}<span class="caption grey--text">/mo</span>
                            <div class="caption green--text">recommended: ${{ c.allocation_costs_recommended.toFixed(2) }}/mo</div>
                        </td>
                    </tr>
                </tbody>
            </v-simple-table>

            <v-simple-table dense class="table mt-5">
                <thead>
                    <tr>
                        <th class="text-left">Instance</th>
                        <th class="text-right">CPU Usage</th>
                        <th></th>
                        <th class="text-right">Memory Usage</th>
                        <th></th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="i in application.instances" :key="i.name">
                        <td class="text-left">{{ i.name }}</td>
                        <td style="width: 150px">
                            <v-sparkline
                                v-if="i.cpu_usage"
                                :value="i.cpu_usage.map((v) => (v === null ? 0 : v)).concat([0])"
                                height="30"
                                width="150"
                                fill
                                padding="4"
                            />
                        </td>
                        <td class="text-right">{{ i.cpu_usage_avg }}</td>
                        <td style="width: 150px">
                            <v-sparkline
                                v-if="i.memory_usage"
                                :value="i.memory_usage.map((v) => (v === null ? 0 : v)).concat([0])"
                                height="30"
                                width="150"
                                fill
                                padding="4"
                            />
                        </td>
                        <td class="text-right">{{ i.memory_usage_avg }}</td>
                    </tr>
                </tbody>
            </v-simple-table>
        </div>
    </div>
</template>

<script>
export default {
    props: {
        applications: Array,
    },

    data() {
        return {
            category: null,
            application: null,
            search: '',
        };
    },

    computed: {
        categories() {
            const cs = new Map();
            this.applications.forEach((a) => {
                let c = cs.get(a.category);
                if (!c) {
                    c = {
                        name: a.category,
                        usage_costs: 0,
                        allocation_costs: 0,
                        over_provisioning_costs: 0,
                        cross_az_traffic_costs: 0,
                        internet_egress_costs: 0,
                    };
                }
                c.usage_costs += a.usage_costs;
                c.allocation_costs += a.allocation_costs;
                if (a.over_provisioning_costs > 0) {
                    c.over_provisioning_costs += a.over_provisioning_costs;
                }
                if (a.cross_az_traffic_costs > 0) {
                    c.cross_az_traffic_costs += a.cross_az_traffic_costs;
                }
                if (a.internet_egress_costs > 0) {
                    c.internet_egress_costs += a.internet_egress_costs;
                }
                cs.set(c.name, c);
            });
            return Array.from(cs.values());
        },
        categoriesTotal() {
            const res = { usage_costs: 0, allocation_costs: 0, over_provisioning_costs: 0, cross_az_traffic_costs: 0, internet_egress_costs: 0 };
            this.categories.forEach((c) => {
                res.usage_costs += c.usage_costs;
                res.allocation_costs += c.allocation_costs;
                res.over_provisioning_costs += c.over_provisioning_costs;
                res.cross_az_traffic_costs += c.cross_az_traffic_costs;
                res.internet_egress_costs += c.internet_egress_costs;
            });
            return res;
        },
        filteredApplications() {
            return this.applications.filter((a) => !this.category || a.category === this.category);
        },
    },

    watch: {
        category: 'filtersToUri',
        application: 'filtersToUri',
        search: 'filtersToUri',
    },

    mounted() {
        this.filtersFromUri();
    },

    methods: {
        filtersToUri() {
            const s = {
                c: this.category !== null ? this.category : undefined,
                a: this.application ? this.application.id : undefined,
                s: this.search || undefined,
            };
            this.$utils.stateToUri(s);
        },
        filtersFromUri() {
            const s = this.$utils.stateFromUri();
            this.category = s.c === undefined ? null : s.c;
            this.application = this.applications.find((a) => a.id === s.a) || null;
            this.search = s.s || '';
        },
        back() {
            this.category = null;
            this.application = null;
            this.search = '';
        },
        searchApplication(value, search, item) {
            return !search || item.id.indexOf(search) !== -1;
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
