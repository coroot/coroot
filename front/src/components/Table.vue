<template>
    <v-simple-table dense>
        <thead>
        <tr>
            <th class="text-left" v-for="h in header">{{h}}</th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="r in rows" :id="r.id" :class="{hi: r.id && $route.hash && $route.hash === '#'+r.id}">
            <td v-for="c in r.cells" class="py-1">
                <v-progress-linear v-if="c.progress"
                                   :background-color='c.progress.color + " lighten-4"'
                                   height="16"
                                   :color='c.progress.color + " lighten-1"'
                                   :value="c.progress.percent"
                                   style="min-width: 64px"
                >
                    <span style="font-size: 14px">{{ c.progress.percent }}%</span>
                </v-progress-linear>

                <div v-else-if="c.net_interfaces" v-for="iface in c.net_interfaces">
                    <span class="text-no-wrap">
                        <v-icon small color="green">mdi-arrow-down-thick</v-icon>{{iface.Rx}}
                    </span>
                    <span class="text-no-wrap">
                        <v-icon small color="blue">mdi-arrow-up-thick</v-icon>{{iface.Tx}}
                        <span class="caption grey--text">({{iface.Name}})</span>
                    </span>
                </div>

                <v-sparkline v-else-if="c.chart" :value="c.chart.map((v) => v === null ? 0 : v)" fill smooth padding="4" color="blue lighten-2" height="32" style="min-width: 100px" />

                <div v-else-if="c.values" v-for="v in c.values">
                    {{v}}
                </div>

                <div v-else-if="c.deployment_summaries" v-for="s in c.deployment_summaries" class="d-flex">
                    <span class="text-no-wrap">{{s.ok ? '&#127881;' : '&#128148;'}} {{s.message}}</span>
                    <router-link :to="{name: 'application', params: {report: s.report}, query: {from: s.time-1800000, to: s.time+1800000}}" class="d-flex">
                        <v-icon small>mdi-chart-box-outline</v-icon>
                    </router-link>
                </div>

                <div v-else class="d-flex">
                    <div>
                        <v-icon v-if="c.icon" :color="c.icon.color" small class="mr-1">{{c.icon.name}}</v-icon>
                        <Led v-if="c.status && c.value" :status="c.status" />
                    </div>
                    <div>
                        <router-link v-if="c.value && c.link" :to="{...{query: $route.query}, ...c.link}">{{c.value}}</router-link>
                        <span v-else :class="{'grey--text': c.is_stub}">{{(smallScreen && c.short_value ? c.short_value : c.value) || '&mdash;'}}</span>
                        <span v-if="c.unit && c.value" class="caption grey--text ml-1">{{c.unit}}</span>
                        <div v-if="c.tags && !smallScreen">
                            <span v-for="t in c.tags" class="tag">{{t}}</span>
                        </div>
                    </div>
                </div>
            </td>
        </tr>
        </tbody>
    </v-simple-table>
</template>

<script>
import Led from "./Led";

export default {
    props: {
        header: Array,
        rows: Array,
    },

    components: {Led},

    computed: {
        smallScreen() {
            return this.$vuetify.breakpoint.xsOnly;
        },
    },
}
</script>

<style scoped>
.hi {
    background-color: #cbe9fc;
}
.tag {
    font-size: 0.75rem;
    color: #9E9E9E;
}
.tag:not(:last-child):after {
    content: " / ";
}
</style>
