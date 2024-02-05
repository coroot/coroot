<template>
    <div>
        <ApplicationFilter :applications="applications" :configureTo="categoriesTo" @filter="setFilter" class="my-4" />

        <div class="legend mb-3">
            <div v-for="s in statuses" class="item">
                <div class="count lighten-1" :class="s.color">{{ s.count }}</div>
                <div class="label">{{ s.name }}</div>
            </div>
        </div>

        <v-data-table
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="50"
            :items="items"
            :headers="[
                { value: 'application', text: 'Application', sortable: false },
                { value: 'errors', text: 'Errors', sortable: false, align: 'end' },
                { value: 'latency', text: 'Latency', sortable: false, align: 'end' },
                { value: 'upstreams', text: 'Upstreams', sortable: false, align: 'end' },
                { value: 'instances', text: 'Instances', sortable: false, align: 'end' },
                { value: 'restarts', text: 'Restarts', sortable: false, align: 'end' },
                { value: 'cpu', text: 'CPU', sortable: false, align: 'end' },
                { value: 'memory', text: 'Mem', sortable: false, align: 'end' },
                { value: 'disk_io', text: 'I/O', sortable: false, align: 'end' },
                { value: 'disk_usage', text: 'Disk', sortable: false, align: 'end' },
                { value: 'network', text: 'Net', sortable: false, align: 'end' },
                { value: 'logs', text: 'Logs', sortable: false, align: 'center' },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
        >
            <template #item.application="{ item: { id, name, ns, color } }">
                <div class="application">
                    <div class="status lighten-1" :class="color" />
                    <div class="name">
                        <router-link :to="link(id, undefined)">{{ name }}</router-link>
                        <span v-if="ns" class="caption grey--text"> (ns:{{ ns }})</span>
                    </div>
                </div>
            </template>
            <template #item.errors="{ item: { id, errors: param } }">
                <router-link :to="link(id, 'SLO')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.latency="{ item: { id, latency: param } }">
                <router-link :to="link(id, 'SLO')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.upstreams="{ item: { id, upstreams: param } }">
                <router-link :to="link(id, 'SLO')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.instances="{ item: { id, instances: param } }">
                <router-link :to="link(id, 'Instances')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.restarts="{ item: { id, restarts: param } }">
                <router-link :to="link(id, 'Instances')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.cpu="{ item: { id, cpu: param } }">
                <router-link :to="link(id, 'CPU')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.memory="{ item: { id, memory: param } }">
                <router-link :to="link(id, 'Memory')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.disk_io="{ item: { id, disk_io: param } }">
                <router-link :to="link(id, 'Storage')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.disk_usage="{ item: { id, disk_usage: param } }">
                <router-link :to="link(id, 'Storage')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.network="{ item: { id, network: param } }">
                <router-link :to="link(id, 'Net')" class="value" :class="param.status">{{ param.value || '–' }}</router-link>
            </template>
            <template #item.logs="{ item: { id, logs: param } }">
                <router-link :to="link(id, 'Logs', { query: JSON.stringify({ source: 'agent', view: 'patterns' }) })">
                    <div class="logs">
                        <div class="value">{{ param.value || '–' }}</div>
                        <v-sparkline
                            v-if="param.chart"
                            :value="param.chart.map((v) => (v === null ? 0 : v))"
                            fill
                            smooth
                            padding="4"
                            color="blue lighten-4"
                            height="30"
                            width="120"
                        />
                    </div>
                </router-link>
            </template>
        </v-data-table>
    </div>
</template>

<script>
import ApplicationFilter from '../components/ApplicationFilter.vue';

const statuses = {
    critical: { name: 'SLO violation', color: 'red' },
    warning: { name: 'Warning', color: 'orange' },
    info: { name: 'Errors in logs', color: 'blue' },
    ok: { name: 'OK', color: 'green' },
};

export default {
    props: {
        applications: Array,
        categoriesTo: Object,
    },

    components: { ApplicationFilter },

    data() {
        return {
            filter: new Set(),
        };
    },

    computed: {
        categories() {
            return Array.from(new Set((this.applications || []).map((a) => a.category)).values());
        },
        statuses() {
            return Object.keys(statuses).map((s) => {
                return {
                    ...statuses[s],
                    count: this.items.filter((i) => i.status === s).length,
                };
            });
        },
        items() {
            if (!this.applications) {
                return [];
            }

            const applications = this.applications.filter((a) => {
                return this.filter.has(a.id);
            });
            const names = {};
            applications.forEach((a) => {
                const id = this.$utils.appId(a.id);
                a.name = id.name;
                a.ns = id.ns;
                if (names[id.name]) {
                    names[id.name]++;
                } else {
                    names[id.name] = 1;
                }
            });
            return applications.map((a) => {
                return {
                    ...a,
                    ns: names[a.name] > 1 ? a.ns : '',
                    color: statuses[a.status].color,
                };
            });
        },
    },

    methods: {
        setFilter(filter) {
            this.filter = filter;
        },
        link(id, report, query) {
            return { name: 'application', params: { id, report }, query: { ...query, ...this.$utils.contextQuery() } };
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
.table:deep(th),
.table:deep(td) {
    padding: 4px 8px !important;
}
.table:deep(td:has(.application)) {
    padding-left: 0 !important;
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
    height: 100%;
    white-space: nowrap;
    display: flex;
    align-items: center;
    justify-content: center;
}
.table:deep(td:has(.logs)) {
    width: 120px;
    min-width: 120px;
    padding: 0 !important;
}
.value {
    color: rgba(0, 0, 0, 0.6);
}
.value.critical,
.value.warning {
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
    color: rgba(255, 255, 255, 0.8);
}
.legend .label {
    color: rgba(0, 0, 0, 0.6);
}
</style>
