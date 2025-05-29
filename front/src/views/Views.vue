<template>
    <div>
        <v-app-bar app flat>
            <v-container fluid class="py-0 px-0 fill-height flex-nowrap">
                <div class="text-h5 nowrap">
                    <div v-if="$slots.subtitle" class="d-flex flex-nowrap gap-2">
                        <template v-if="$vuetify.breakpoint.smAndUp">
                            <router-link :to="{ name: 'overview', params: { view } }">{{ title }}</router-link>
                            <v-icon>mdi-chevron-right</v-icon>
                        </template>
                        <div class="text-h6 font-weight-regular nowrap">
                            <slot name="subtitle"></slot>
                        </div>
                    </div>
                    <template v-else>{{ title }}</template>
                </div>
                <v-spacer />

                <div class="ml-3">
                    <TimePicker :small="$vuetify.breakpoint.xsOnly" />
                </div>
            </v-container>
        </v-app-bar>

        <v-progress-linear v-if="loading" indeterminate height="2" color="success" style="position: absolute; top: 0; left: 0" />

        <v-alert v-if="error" color="error" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <slot v-else></slot>
    </div>
</template>

<script>
import TimePicker from '@/components/TimePicker.vue';

export const views = {
    applications: { name: 'Applications', icon: 'mdi-apps' },
    incidents: { name: 'Incidents', icon: 'mdi-alert-outline' },
    map: { name: 'Service Map', icon: 'mdi-map-outline' },
    traces: { name: 'Traces', icon: 'mdi-chart-timeline' },
    logs: { name: 'Logs', icon: 'mdi-text-search' },
    nodes: { name: 'Nodes', icon: 'mdi-server' },
    deployments: { name: 'Deployments', icon: 'mdi-rocket-launch-outline' },
    costs: { name: 'Costs', icon: 'mdi-currency-usd' },
    anomalies: { name: 'Anomalies', icon: 'mdi-waveform' },
    risks: { name: 'Risks', icon: 'mdi-weather-lightning' },
    dashboards: { name: 'Dashboards', icon: 'mdi-view-dashboard-outline' },
};

if (window.coroot.edition !== 'Enterprise') {
    delete views.anomalies;
}

export default {
    props: {
        loading: Boolean,
        error: String,
    },

    components: { TimePicker },

    computed: {
        view() {
            return this.$route.params.view;
        },
        title() {
            const v = views[this.view];
            if (!v) {
                return null;
            }
            return v.name;
        },
    },
};
</script>

<style scoped></style>
