<template>
    <v-simple-table>
        <thead>
        <tr>
            <th class="text-left" v-for="h in header">{{h}}</th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="r in rows">
            <td v-for="c in r.cells" class="py-2">
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

                <v-sparkline v-else-if="c.chart" :value="c.chart" fill smooth padding="4" color="blue lighten-2" height="40" style="min-width: 100px" />

                <div v-else-if="c.values" v-for="v in c.values">
                    {{v}}
                </div>

                <div v-else-if="c.deployment_summaries" v-for="s in c.deployment_summaries" class="d-flex">
                    <v-icon v-if="s.ok" small color="hsl(141, 71%, 48%)">mdi-check-circle-outline</v-icon>
                    <v-icon v-else small color="hsl(4, 90%, 58%)">mdi-close-circle-outline</v-icon>
                    <span class="mx-1">{{s.message}}</span>
                    <router-link :to="{name: 'application', params: {report: s.report}, query: {from: s.time-1800000, to: s.time+1800000}}" class="d-flex">
                        <v-icon small>mdi-chart-box-outline</v-icon>
                    </router-link>
                </div>

                <template v-else>
                    <v-icon v-if="c.icon" :color="c.icon.color" small class="mr-1">{{c.icon.name}}</v-icon>
                    <Led v-if="c.status && c.value" :status="c.status" />
                    <template v-if="c.value && c.link">
                        <router-link v-if="c.link.type === 'application'" :to="{name: 'application', params: {id: c.link.key}, query: $route.query}">{{c.value}}</router-link>
                        <router-link v-else-if="c.link.type === 'node'" :to="{name: 'node', params: {name: c.link.key}, query: $route.query}">{{c.value}}</router-link>
                    </template>
                    <span v-else :class="{'grey--text': c.is_stub}">{{c.value || '&mdash;'}}</span>
                    <span v-if="c.unit && c.value" class="caption grey--text ml-1">{{c.unit}}</span>
                    <div v-if="c.tags && $vuetify.breakpoint.smAndUp" :class="{'pl-4': c.status}">
                        <span v-for="t in c.tags" class="tag">{{t}}</span>
                    </div>
                </template>
            </td>
        </tr>
        </tbody>
    </v-simple-table>
</template>

<script>
import Led from "@/components/Led";

export default {
    props: {
        header: Array,
        rows: Array,
    },

    components: {Led},
}
</script>

<style scoped>
.tag {
    font-size: 0.75rem;
    color: #9E9E9E;
}
.tag:not(:last-child):after {
    content: " / ";
}
</style>
