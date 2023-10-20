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

    <div class="legend mb-3">
        <div v-for="s in statuses" class="item">
            <div class="count lighten-1" :class="s.color">{{s.count}}</div>
            <div class="label">{{s.name}}</div>
        </div>
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
            <div class="application">
                <div class="status lighten-1" :class="item.color" />
                <div class="name">
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
            <div class="logs">
                <div class="value">{{item.logs.value}}</div>
                <v-sparkline v-if="item.logs.chart" :value="item.logs.chart.map((v) => v === null ? 0 : v)" fill smooth padding="4" color="blue lighten-4" height="60" class="chart" />
            </div>
        </template>
    </v-data-table>
</div>
</template>

<script>
import ApplicationCategories from "../components/ApplicationCategories.vue";

const statuses = {
    critical: {name: 'SLO violation', color: 'red'},
    warning: {name: 'Warning', color: 'orange'},
    info: {name: 'Errors in logs', color: 'blue'},
    ok: {name: 'OK', color: 'green'},
};

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
            return Array.from(new Set((this.applications || []).map(a => a.category)).values());
        },
        statuses() {
            return Object.keys(statuses).map(s => {
                return {
                    ...statuses[s],
                    count: this.items.filter(i => i.status === s).length,
                }
            })
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
                    color: statuses[a.status].color,
                }
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
.table:deep(td):last-child {
    padding-right: 0 !important;
}
.table .application {
    display: flex;
    gap: 4px;
}
.table .application .status {
    height: 20px;
    width: 4px;
}
.table .application .name {
    max-width: 60ch;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.table .logs {
    position: relative;
    height: 100%;
}
.table .logs .value {
    position: absolute;
    top: 0;
    width: 100%;
    white-space: nowrap;
    text-align: center;
    color: rgba(0,0,0,0.6);
}
.table .logs .chart {
    min-width: 16ch;
}
.value {
    color: rgba(0,0,0,0.6);
}
.value.critical, .value.warning {
    color: inherit;
    border-bottom: 2px solid red !important;
    background-color: unset !important;
}
.legend {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    align-items: center;
    font-weight: 500;
    font-size: 14px;
}
.legend .item {
    display: flex;
    gap: 4px;
}
.legend .count {
    padding: 0 4px;
    border-radius: 2px;
    height: 18px;
    line-height: 18px;
    color: rgba(255,255,255,0.8)
}
.legend .label {
    color: rgba(0,0,0,0.6);
}
</style>