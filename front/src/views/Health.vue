<template>
<div>
    <v-row class="my-4">
        <v-col cols="12" sm="3">
            <v-text-field v-model="search" dense hide-details clearable prepend-inner-icon="mdi-magnify" label="Search" single-line outlined class="search" />
        </v-col>
        <v-col class="d-flex">
            <v-spacer />
            <ApplicationCategories :categories="categories" :configureTo="categoriesTo" @change="setSelectedCategories" />
        </v-col>
    </v-row>

    <div class="d-flex flex-wrap mb-3" style="gap: 8px">
        <v-chip v-for="s in statuses" label :color="s.color" dark small class="pl-3 pr-1">
            <div class="font-weight-bold mr-2 text-uppercase">{{s.name}}</div>
            <div class="tag darken-3" :class="s.color">{{s.count}}</div>
        </v-chip>
    </div>

    <v-data-table dense class="table" mobile-breakpoint="0" :items-per-page="50"
        :items="items"
        :headers="[
            {value: 'application', text: 'Application', sortable: false},
            {value: 'errors', text: 'Errors', sortable: false, align: 'end'},
            {value: 'latency', text: 'Latency', sortable: false, align: 'end'},
            {value: 'instances', text: 'Instances', sortable: false, align: 'end'},
            {value: 'restarts', text: 'Restarts', sortable: false, align: 'end'},
            {value: 'cpu', text: 'CPU', sortable: false, align: 'end'},
            {value: 'memory', text: 'Mem', sortable: false, align: 'end'},
            {value: 'disk_io', text: 'I/O', sortable: false, align: 'end'},
            {value: 'disk_usage', text: 'Disk', sortable: false, align: 'end'},
            {value: 'network', text: 'Net', sortable: false, align: 'end'},
            {value: 'logs', text: 'Logs', sortable: false, align: 'center'},
        ]"
        :footer-props="{itemsPerPageOptions: [10, 20, 50, 100, -1]}"
    >
        <template #item.application="{item}">
            <div class="d-flex" style="gap: 4px">
                <div class="marker" :class="item.color" />
                <div class="text-no-wrap">
                    <router-link :to="{name: 'application', params: {id: item.id}, query: $utils.contextQuery()}">
                        {{ $utils.appId(item.id).name }}
                    </router-link>
                </div>
            </div>
        </template>
        <template #item.errors="{item}">
            <span class="value" :class="item.errors.status">{{item.errors.value || '–'}}</span>
        </template>
        <template #item.latency="{item}">
            <span class="value" :class="item.latency.status">{{item.latency.value || '–'}}</span>
        </template>
        <template #item.instances="{item}">
            <span class="value" :class="item.instances.status">{{item.instances.value || '–'}}</span>
        </template>
        <template #item.restarts="{item}">
            <span class="value" :class="item.restarts.status">{{item.restarts.value || '–'}}</span>
        </template>
        <template #item.cpu="{item}">
            <span class="value" :class="item.cpu.status">{{item.cpu.value || '–'}}</span>
        </template>
        <template #item.memory="{item}">
            <span class="value" :class="item.memory.status">{{item.memory.value || '–'}}</span>
        </template>
        <template #item.disk_io="{item}">
            <span class="value" :class="item.disk_io.status">{{item.disk_io.value || '–'}}</span>
        </template>
        <template #item.disk_usage="{item}">
            <span class="value" :class="item.disk_usage.status">{{item.disk_usage.value || '–'}}</span>
        </template>
        <template #item.network="{item}">
            <span class="value" :class="item.network.status">{{item.network.value || '–'}}</span>
        </template>
        <template #item.logs="{item}">
            <div style="position: relative; height: 100%">
                <div class="text-center text-no-wrap font-weight-light" style="position: absolute;top:0;width: 100%">{{item.logs.value}}</div>
                <v-sparkline v-if="item.logs.chart" :value="item.logs.chart.map((v) => v === null ? 0 : v)" fill smooth padding="4" color="blue lighten-4" height="60" style="min-width: 80px" />
            </div>
        </template>
        <template #no-data>
            No issues detected
        </template>
    </v-data-table>
</div>
</template>

<script>
import ApplicationCategories from "../components/ApplicationCategories.vue";
import {statuses as colors} from "../utils/colors";

export default {
    props: {
        applications: Array,
        categoriesTo: Object,
    },

    components: {ApplicationCategories},

    data() {
        return {
            selectedCategories: new Set(),
            search: '',
        };
    },

    computed: {
        categories() {
            if (!this.applications) {
                return [];
            }
            return Array.from(new Set((this.applications || []).map(a => a.category)).values());
        },
        statuses() {
            const statuses = [
                {id: 'critical', name: 'SLO violation'},
                {id: 'warning', name: 'Warning'},
                {id: 'info', name: 'Errors in logs'},
                {id: 'ok', name: 'OK'},
            ];
            statuses.forEach(s => {
                s.color = colors[s.id];
                s.count = this.items.filter(i => i.status === s.id).length;
            });
            return statuses;
        },
        items() {
            if (!this.applications) {
                return [];
            }
            return this.applications.filter(a => {
                if (!this.selectedCategories.has(a.category)) {
                    return false;
                }
                if (this.search && !a.id.includes(this.search)) {
                    return false;
                }
                return true;
            }).map(a => {
                return {
                    ...a,
                    color: colors[a.status],
                };
            });
        },
    },

    methods: {
        setSelectedCategories(categories) {
            this.selectedCategories = new Set(categories);
        },
    },
}
</script>

<style scoped>
.table:deep(table) {
    min-width: 500px;
}
.table:deep(tr:hover) {
    background-color: unset !important;
}
.table:deep(th), .table:deep(td) {
    padding: 4px 8px !important;
}
.table:deep(td):first-child {
    padding-left: 0 !important;
}
.marker {
    height: 20px;
    width: 4px;
}
.value {
    color: rgba(0,0,0,0.6);
}
.value.critical, .value.warning {
    color: inherit;
    border-bottom: 2px solid red !important;
    background-color: unset !important;
}
.tag {
    padding: 0 4px;
    border-radius: 2px;
    height: 18px;
    line-height: 18px;
}
</style>