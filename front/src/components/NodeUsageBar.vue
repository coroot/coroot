<template>
    <div class="bar" ref="bar" @mouseleave="leave">
        <div v-for="a in applications" :key="a.name" :style="style(a)" @mouseenter="(e) => enter(a, e)" />
        <div class="flex-grow-1" @mouseenter="(e) => enter(null, e)" />

        <v-tooltip v-if="tooltip" :value="!!tooltip" :position-x="tooltip.x" :position-y="tooltip.y" bottom transition="none">
            <v-card class="pa-3">
                <div class="font-weight-medium">
                    {{ tooltip.app.name }}
                    <span class="caption grey--text">{{ tooltip.app.value.toFixed(0) }}%</span>
                </div>
                <v-simple-table v-if="tooltip.app.instances && tooltip.app.instances.length" dense class="mt-3">
                    <thead>
                        <tr>
                            <th class="text-left">Instance</th>
                            <th></th>
                            <th class="text-right">Usage</th>
                            <th class="text-right">Request</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="i in tooltip.app.instances" :key="i.name">
                            <td class="text-left">{{ i.name }}</td>
                            <td style="width: 150px">
                                <v-sparkline :value="i.chart.map((v) => (v === null ? 0 : v)).concat([0])" height="30" width="150" fill padding="4" />
                            </td>
                            <td class="text-right">{{ i.usage }}</td>
                            <td class="text-right">
                                <template v-if="i.request">{{ i.request }}</template>
                                <template v-else>&mdash;</template>
                            </td>
                        </tr>
                    </tbody>
                </v-simple-table>
            </v-card>
        </v-tooltip>
    </div>
</template>

<script>
import { palette } from '../utils/colors';

export default {
    props: {
        applications: Array,
    },

    data() {
        return {
            tooltip: null,
        };
    },

    methods: {
        style(a) {
            let color = palette.hash(a.name);
            if (a.name === '~other') {
                color = 'rgba(243,219,160)';
            }
            if (a.name === '~cached') {
                color = 'rgb(196, 196, 196)';
            }
            return {
                width: a.value + '%',
                backgroundColor: color,
            };
        },
        enter(a, e) {
            if (!a) {
                a = { name: '~idle', value: this.applications.reduce((s, a) => s - a.value, 100) };
            }
            const rect = e.target.getBoundingClientRect();
            this.tooltip = { app: a, x: rect.left + rect.width / 2, y: rect.top + rect.height };
        },
        leave() {
            this.tooltip = null;
        },
    },
};
</script>

<style scoped>
.bar {
    display: flex;
    height: 16px;
    background-color: rgba(0, 0, 0, 0.1);
    filter: brightness(var(--brightness));
}
</style>
