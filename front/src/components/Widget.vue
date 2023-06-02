<template>
<div>
    <Chart v-if="w.chart" :chart="w.chart"/>
    <ChartGroup v-if="w.chart_group" :title="w.chart_group.title" :charts="w.chart_group.charts"/>
    <LogPatterns v-if="w.log_patterns" :title="w.log_patterns.title" :patterns="w.log_patterns.patterns" />
    <DependencyMap v-if="w.dependency_map" :nodes="w.dependency_map.nodes" :links="w.dependency_map.links" />
    <Table v-if="w.table" :header="w.table.header" :rows="w.table.rows" />
    <Heatmap v-if="w.heatmap" :heatmap="w.heatmap" :selection="heatmapSelection" @select="heatmapDrillDown" />
    <Profile v-if="w.profile" :appId="w.profile.application_id" />
    <Tracing v-if="w.tracing" :appId="w.tracing.application_id" />
</div>
</template>

<script>
import Chart from "./Chart";
import ChartGroup from "./ChartGroup";
import LogPatterns from "./LogPatterns";
import DependencyMap from "./DependencyMap";
import Table from "./Table";
import Heatmap from "./Heatmap";
import Profile from "../views/Profile";
import Tracing from "../views/Tracing";

export default {
    props: {
        w: Object,
    },

    components: {Chart, ChartGroup, LogPatterns, DependencyMap, Table, Heatmap, Profile, Tracing},

    computed: {
        heatmapSelection() {
            const hm = this.w.heatmap;
            return hm && hm.drill_down_link ? {} : null;
        },
    },

    methods: {
        heatmapDrillDown(s) {
            const hm = this.w.heatmap;
            if (hm && hm.drill_down_link && s.x1) {
                const tsRange = `${s.x1 || ''}-${s.x2 || ''}`;
                const durRange = `${s.y1 || ''}-${s.y2 || ''}`;
                const trace = `::${tsRange}:${durRange}:`;
                const {from, to} = this.w.heatmap.ctx;
                const query = {...this.$route.query, from, to, trace};
                this.$router.push({...hm.drill_down_link, query}).catch(err => err);
            }
        }
    },
}
</script>

<style scoped>

</style>
