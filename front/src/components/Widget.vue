<template>
    <div>
        <Chart v-if="w.chart" :chart="w.chart" :selection="{}" @select="chartZoom" :doc="doc_link" />
        <ChartGroup
            v-if="w.chart_group"
            :title="w.chart_group.title"
            :charts="w.chart_group.charts"
            :selection="{}"
            @select="chartZoom"
            :doc="doc_link"
        />
        <DependencyMap v-if="w.dependency_map" :nodes="w.dependency_map.nodes" :links="w.dependency_map.links" />
        <Table v-if="w.table" :header="w.table.header" :rows="w.table.rows" />
        <Heatmap v-if="w.heatmap" :heatmap="w.heatmap" :selection="heatmapSelection" @select="heatmapDrillDown" />
        <Logs v-if="w.logs" :appId="w.logs.application_id" :check="w.logs.check" />
        <Profiling v-if="w.profiling" :appId="w.profiling.application_id" />
        <Tracing v-if="w.tracing" :appId="w.tracing.application_id" />
    </div>
</template>

<script>
import Chart from './Chart';
import ChartGroup from './ChartGroup';
import DependencyMap from './DependencyMap';
import Table from './Table';
import Heatmap from './Heatmap';
import Logs from '../views/Logs';
import Profiling from '../views/Profiling';
import Tracing from '../views/Tracing';

export default {
    props: {
        w: Object,
    },

    components: { Chart, ChartGroup, DependencyMap, Table, Heatmap, Logs, Profiling, Tracing },

    computed: {
        heatmapSelection() {
            const hm = this.w.heatmap;
            return hm && hm.drill_down_link ? {} : null;
        },
        doc_link() {
            const l = this.w.doc_link;
            if (!l) {
                return null;
            }
            return `https://docs.coroot.com/${l.group}/${l.item}${l.hash ? '#' + l.hash : ''}`;
        },
    },

    methods: {
        chartZoom(s) {
            const { from, to } = s.selection;
            const query = { ...this.$route.query, from, to };
            this.$router.push({ query }).catch((err) => err);
        },
        heatmapDrillDown(s) {
            const hm = this.w.heatmap;
            if (hm && hm.drill_down_link && s.x1) {
                const tsRange = `${s.x1 || ''}-${s.x2 || ''}`;
                const durRange = `${s.y1 || ''}-${s.y2 || ''}`;
                const trace = `::${tsRange}:${durRange}:`;
                const { from, to } = this.w.heatmap.ctx;
                const query = { ...this.$route.query, from, to, trace };
                this.$router.push({ ...hm.drill_down_link, query }).catch((err) => err);
            }
        },
    },
};
</script>

<style scoped></style>
