<template>
    <v-simple-table>
        <thead>
        <tr>
            <th class="text-left" v-for="h in header">{{h}}</th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="r in rows">
            <td v-for="c in r.cells">
                <v-icon v-if="c.icon" :color="c.icon.color" small class="mr-1">{{c.icon.name}}</v-icon>
                <Led v-if="c.status && c.value" :status="c.status" class="mr-1" />
                <div v-for="v in c.values">
                    {{v}}
                </div>
                <span v-if="!c.values">
                    <router-link v-if="c.link === 'node'" :to="{name: 'node', params: {name: c.value}, query: $route.query}">{{c.value}}</router-link>
                    <span v-else-if="!c.progress">{{c.value || '&mdash;'}}</span>
                </span>
                <span v-if="c.unit && c.value" class="caption grey--text ml-1">{{c.unit}}</span>
                <div v-if="c.tags">
                    <span v-for="t in c.tags" class="tag">{{t}}</span>
                </div>
                <v-progress-linear v-if="c.progress"
                    :background-color='c.progress.color + " lighten-4"'
                    height="16"
                    :color='c.progress.color + " lighten-1"'
                    :value="c.progress.percent"
                    style="min-width: 64px"
                >
                    <span style="font-size: 14px">{{ c.progress.percent }}%</span>
                </v-progress-linear>
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
