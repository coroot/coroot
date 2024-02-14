<template>
    <Chart :chart="chart" :selection="selection" @select="select">
        <template v-slot:title>
            <span>{{ splitTitle.head }}</span>
            <v-menu offset-y>
                <template #activator="{ on, attrs }">
                    <v-btn v-bind="attrs" v-on="on" text outlined x-small class="selector">
                        <span style="max-width: 90%; overflow: hidden; text-overflow: ellipsis">{{ selected }}</span>
                        <v-icon small class="ml-1">mdi-menu-down</v-icon>
                    </v-btn>
                </template>
                <v-list dense class="pa-0">
                    <v-list-item-group :value="selected">
                        <v-list-item v-for="ch in sorted" :key="ch.title" @click="selected = ch.title" class="py-1 px-2" style="min-height: 0">
                            <v-list-item-title class="item">{{ ch.title }}</v-list-item-title>
                        </v-list-item>
                    </v-list-item-group>
                </v-list>
            </v-menu>
            <span>{{ splitTitle.tail }}</span>
            <a v-if="doc" :href="doc" target="_blank" class="ml-1"><v-icon small>mdi-information-outline</v-icon></a>
        </template>
    </Chart>
</template>

<script>
import Chart from './Chart';

export default {
    props: {
        title: String,
        charts: Array,
        selection: Object,
        doc: String,
    },

    components: { Chart },

    data() {
        const charts = this.sort();
        const i = charts.findIndex((ch) => ch.featured);
        return {
            selected: charts[i < 0 ? 0 : i].title,
        };
    },

    computed: {
        chart() {
            let chart = this.sorted.find((ch) => ch.title === this.selected);
            if (chart) return chart;
            chart = this.sorted.find((ch) => ch.featured);
            if (chart) return chart;
            return this.sorted[0];
        },
        sorted() {
            return this.sort();
        },
        splitTitle() {
            const parts = this.title.split('<selector>', 2);
            if (parts.length === 0) {
                return { head: '', tail: '' };
            }
            if (parts.length === 1) {
                return { head: parts[0], tail: '' };
            }
            return { head: parts[0], tail: parts[1] };
        },
    },

    methods: {
        sort() {
            const res = Array.from(this.charts);
            res.sort((a, b) => a.title.localeCompare(b.title));
            return res;
        },
        select(s) {
            this.$emit('select', s);
        },
    },
};
</script>

<style scoped>
.selector {
    font-size: 14px;
    display: inline;
    max-width: 30%;
    padding: 0 4px !important;
    border-color: var(--border-color) !important;
}
.item {
    font-size: 14px !important;
}
</style>
