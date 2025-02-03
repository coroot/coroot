<template>
    <v-simple-table dense>
        <thead>
            <tr>
                <th class="text-left" v-for="h in header">{{ h }}</th>
            </tr>
        </thead>
        <tbody>
            <tr v-for="r in rows" :id="r.id" :class="{ hi: r.id && $route.hash && $route.hash === '#' + r.id }">
                <td v-for="c in r.cells" class="py-1">
                    <v-progress-linear
                        v-if="c.progress"
                        :background-color="c.progress.color + ' lighten-3'"
                        height="16"
                        :color="c.progress.color + ' lighten-1'"
                        :value="c.progress.percent"
                        style="min-width: 64px"
                    >
                        <span style="font-size: 14px">{{ c.progress.percent }}%</span>
                    </v-progress-linear>

                    <div v-else-if="c.bandwidth">
                        <span class="text-no-wrap"> <v-icon small color="green">mdi-arrow-down-thick</v-icon>{{ c.bandwidth.Rx }} </span>
                        <span class="text-no-wrap"> <v-icon small color="blue">mdi-arrow-up-thick</v-icon>{{ c.bandwidth.Tx }} </span>
                    </div>

                    <v-sparkline
                        v-else-if="c.chart"
                        :value="c.chart.map((v) => (v === null ? 0 : v))"
                        fill
                        smooth
                        padding="4"
                        color="blue lighten-2"
                        height="32"
                        style="min-width: 100px"
                    />

                    <template v-else-if="c.values">
                        <v-menu v-if="c.values.length > 1" offset-y tile>
                            <template #activator="{ on }">
                                <span v-on="on" class="text-no-wrap"> {{ c.values[0] }}, ...</span>
                            </template>
                            <v-list dense>
                                <v-list-item v-for="v in c.values" style="font-size: 14px; min-height: 32px">
                                    <v-list-item-title>{{ v }}</v-list-item-title>
                                </v-list-item>
                            </v-list>
                        </v-menu>
                        <span v-else>{{ c.values[0] }}</span>
                    </template>

                    <div v-else-if="c.deployment_summaries" v-for="s in c.deployment_summaries" class="d-flex">
                        <span class="text-no-wrap">{{ s.ok ? '&#127881;' : '&#128148;' }} {{ s.message }}</span>
                        <router-link
                            :to="{
                                name: 'overview',
                                params: { view: 'applications', report: s.report },
                                query: { from: s.time - 1800000, to: s.time + 1800000 },
                            }"
                            class="d-flex"
                        >
                            <v-icon small>mdi-chart-box-outline</v-icon>
                        </router-link>
                    </div>

                    <div v-else>
                        <div class="d-flex">
                            <v-icon v-if="c.icon" :color="c.icon.color" small class="mr-1">{{ c.icon.name }}</v-icon>
                            <Led v-if="c.status && c.value" :status="c.status" />
                            <router-link
                                v-if="c.value && c.link"
                                :to="{ ...{ query: $route.query }, ...c.link }"
                                :class="{ truncated: !!c.max_width }"
                                :style="{ 'max-width': !!c.max_width ? c.max_width + 'ch' : undefined }"
                                :title="c.value"
                                >{{ c.value }}</router-link
                            >
                            <span
                                v-else
                                :class="{ 'grey--text': c.is_stub, truncated: !!c.max_width }"
                                :style="{ 'max-width': !!c.max_width ? c.max_width + 'ch' : undefined }"
                                :title="c.value"
                                >{{ (smallScreen && c.short_value ? c.short_value : c.value) || '&mdash;' }}</span
                            >
                            <span v-if="c.unit && c.value" class="caption grey--text ml-1">{{ c.unit }}</span>
                        </div>
                        <div v-if="c.tags && !smallScreen">
                            <span v-for="t in c.tags" class="tag">{{ t }}</span>
                        </div>
                    </div>
                </td>
            </tr>
        </tbody>
    </v-simple-table>
</template>

<script>
import Led from './Led';

export default {
    props: {
        header: Array,
        rows: Array,
    },

    components: { Led },

    computed: {
        smallScreen() {
            return this.$vuetify.breakpoint.xsOnly;
        },
    },
};
</script>

<style scoped>
.tag {
    font-size: 0.75rem;
    color: #9e9e9e;
}
.tag:not(:last-child):after {
    content: ' / ';
}
.truncated {
    max-width: 30ch;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    display: block;
}
</style>
