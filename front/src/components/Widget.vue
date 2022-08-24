<template>
<div>
    <Chart v-if="w.chart" :chart="w.chart"/>
    <ChartGroup v-if="w.chart_group" :title="w.chart_group.title" :charts="w.chart_group.charts"/>
    <LogPatterns v-if="w.log_patterns" :title="w.log_patterns.title" :patterns="w.log_patterns.patterns" />
    <DependencyMap v-if="w.dependency_map" :nodes="w.dependency_map.nodes" :links="w.dependency_map.links" />
</div>
</template>

<script>
import Chart from "@/components/Chart";
import ChartGroup from "@/components/ChartGroup";
import LogPatterns from "@/components/LogPatterns";
import DependencyMap from "@/components/DependencyMap";

export default {
    props: {
        w: Object,
    },

    components: {Chart, ChartGroup, LogPatterns, DependencyMap},
}
</script>

<style scoped>

</style>
