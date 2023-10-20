<template>
<div>
    <v-row class="my-4">
        <v-col cols="12" sm="3">
            <v-text-field v-model="search" dense hide-details clearable prepend-inner-icon="mdi-magnify" label="Search" single-line outlined class="search" />
        </v-col>
        <v-col class="d-flex">
            <v-spacer />
            <ApplicationCategories :categories="categories" :configureTo="categoriesTo" @change="setSelectedCategories" :disabled="!!search" />
        </v-col>
    </v-row>

    <v-data-table dense class="table" mobile-breakpoint="0" :items-per-page="20"
        :items="items"
        :headers="[
            {value: 'application', text: 'Application', sortable: false},
            {value: 'deployment', text: 'Deployment', sortable: false},
            {value: 'deployed', text: 'Deployed', sortable: false},
            {value: 'summary', text: 'Summary', sortable: false},
        ]"
        :footer-props="{itemsPerPageOptions: [10, 20, 50, 100, -1]}"
    >
        <template #item.application="{item}">
            <div class="text-no-wrap">
                {{ $utils.appId(item.application.id).name }}
            </div>
            <div class="caption grey--text">
                ns: {{ $utils.appId(item.application.id).ns }}
            </div>
        </template>
        <template #item.deployment="{item}">
            <div class="d-flex">
                <Led :status="item.status" />
                <div>
                    <router-link :to="item.link" class="text-no-wrap">
                        {{item.version}}
                    </router-link>
                    <div class="caption grey--text">
                        age: {{item.age}}
                    </div>
                </div>
            </div>
        </template>
        <template #item.deployed="{item}">
            <span class="text-no-wrap">{{item.deployed}}</span>
        </template>
        <template #item.summary="{item}">
            <div v-for="s in item.summary" class="text-no-wrap">
                <span v-if="s.status" class="mr-1">{{s.status}}</span>
                <span :class="{'grey--text': !s.status}">{{s.message}}</span>
                <router-link v-if="s.link" :to="s.link" class="ml-1">
                    <v-icon small>mdi-chart-box-outline</v-icon>
                </router-link>
            </div>
        </template>
        <template #no-data>
            No deployments detected
        </template>
    </v-data-table>
</div>
</template>

<script>
import Led from "../components/Led.vue";
import ApplicationCategories from "../components/ApplicationCategories.vue";

export default {
    props: {
        deployments: Array,
        categoriesTo: Object,
    },

    components: {ApplicationCategories, Led},

    data() {
        return {
            selectedCategories: new Set(),
            search: '',
        };
    },

    computed: {
        categories() {
            return Array.from(new Set((this.deployments || []).map(d => d.application.category)).values());
        },
        items() {
            if (!this.deployments) {
                return [];
            }
            return this.deployments.filter(d => {
                if (this.search) {
                    return d.application.id.includes(this.search);
                }
                return this.selectedCategories.has(d.application.category);
            })
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
    padding: 8px !important;
}
</style>