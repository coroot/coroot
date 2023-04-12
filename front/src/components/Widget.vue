<template>
<div>
    <Chart v-if="w.chart" :chart="w.chart"/>
    <ChartGroup v-if="w.chart_group" :title="w.chart_group.title" :charts="w.chart_group.charts"/>
    <LogPatterns v-if="w.log_patterns" :title="w.log_patterns.title" :patterns="w.log_patterns.patterns" />
    <DependencyMap v-if="w.dependency_map" :nodes="w.dependency_map.nodes" :links="w.dependency_map.links" />
    <Table v-if="w.table" :header="w.table.header" :rows="w.table.rows" />
    <Profile v-if="w.profile" :appId="w.profile.application_id" />
    <Heatmap v-if="w.heatmap" :heatmap="w.heatmap" />
</div>
</template>

<script>
import Chart from "@/components/Chart";
import ChartGroup from "@/components/ChartGroup";
import LogPatterns from "@/components/LogPatterns";
import DependencyMap from "@/components/DependencyMap";
import Table from "@/components/Table";
import Profile from "@/views/Profile";
import Heatmap from "@/components/Heatmap";

export default {
    props: {
        w: Object,
    },

    components: {Chart, ChartGroup, LogPatterns, DependencyMap, Table, Profile, Heatmap},
}
</script>

<style scoped>

</style>
