<template>
    <div>
        <v-progress-linear indeterminate v-if="loading" color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <ApplicationFilter :applications="applications" @filter="setFilter" class="my-4" />

        <v-data-table
            dense
            class="table"
            mobile-breakpoint="0"
            :items-per-page="20"
            :items="items"
            :headers="[
                { value: 'application', text: 'Application', sortable: false },
                { value: 'deployment', text: 'Deployment', sortable: false },
                { value: 'deployed', text: 'Deployed', sortable: false },
                { value: 'summary', text: 'Summary', sortable: false },
            ]"
            :footer-props="{ itemsPerPageOptions: [10, 20, 50, 100, -1] }"
        >
            <template #item.application="{ item }">
                <div class="text-no-wrap">
                    {{ $utils.appId(item.application.id).name }}
                </div>
                <div class="caption grey--text">ns: {{ $utils.appId(item.application.id).ns }}</div>
            </template>
            <template #item.deployment="{ item }">
                <div class="d-flex">
                    <Led :status="item.status" />
                    <div>
                        <router-link :to="item.link" class="text-no-wrap">
                            {{ item.version }}
                        </router-link>
                        <div class="caption grey--text">age: {{ item.age }}</div>
                    </div>
                </div>
            </template>
            <template #item.deployed="{ item }">
                <span class="text-no-wrap">{{ item.deployed }}</span>
            </template>
            <template #item.summary="{ item }">
                <div v-for="s in item.summary" class="text-no-wrap">
                    <span v-if="s.status" class="mr-1">{{ s.status }}</span>
                    <span :class="{ 'grey--text': !s.status }">{{ s.message }}</span>
                    <router-link v-if="s.link" :to="s.link" class="ml-1">
                        <v-icon small>mdi-chart-box-outline</v-icon>
                    </router-link>
                </div>
            </template>
            <template #no-data> No deployments detected </template>
        </v-data-table>
    </div>
</template>

<script>
import Led from '../components/Led.vue';
import ApplicationFilter from '../components/ApplicationFilter.vue';

export default {
    components: { ApplicationFilter, Led },

    data() {
        return {
            deployments: [],
            filter: new Set(),
            error: '',
            loading: false,
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    computed: {
        applications() {
            if (!this.deployments) {
                return [];
            }
            const applications = {};
            this.deployments.forEach((d) => {
                applications[d.application.id] = d.application.category;
            });
            return Object.keys(applications).map((id) => ({ id, category: applications[id] }));
        },
        items() {
            if (!this.deployments) {
                return [];
            }
            return this.deployments.filter((d) => {
                return this.filter.has(d.application.id);
            });
        },
    },

    methods: {
        setFilter(filter) {
            this.filter = filter;
        },
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getOverview('deployments', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.deployments = data.deployments;
            });
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
    padding: 8px !important;
}
</style>
