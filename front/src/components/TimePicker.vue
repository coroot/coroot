<template>
    <v-menu close-on-content-click offset-y>
        <template #activator="{ on, attrs }">
            <v-btn v-on="on" plain outlined height="40" class="px-2">
                <v-icon>mdi-clock-outline</v-icon>
                <span v-if="!small" class="ml-2">{{ intervals.find((i) => i.active).text }}</span>
                <v-icon v-if="!small" small class="ml-2"> mdi-chevron-{{ attrs['aria-expanded'] === 'true' ? 'up' : 'down' }} </v-icon>
            </v-btn>
        </template>
        <v-list dense dark>
            <v-list-item v-for="i in intervals" :key="i.text" :to="{ query: i.query }" exact>
                {{ i.text }}
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script>
export default {
    props: {
        small: Boolean,
    },

    computed: {
        intervals() {
            const intervals = [
                { text: 'last hour', query: {} },
                { text: 'last 3 hours', query: { from: 'now-3h' } },
                { text: 'last 12 hours', query: { from: 'now-12h' } },
                { text: 'last day', query: { from: 'now-1d' } },
                { text: 'last 3 days', query: { from: 'now-3d' } },
                { text: 'last week', query: { from: 'now-7d' } },
            ];
            const incident = this.$route.query.incident;
            if (incident) {
                intervals.unshift({ text: 'incident: ' + incident, query: { incident }, active: true });
                return intervals;
            }
            const from = this.$route.query.from;
            const to = this.$route.query.to === 'now' ? undefined : this.$route.query.to;
            const selected = intervals.find((i) => i.query.from === from && i.query.to === to);
            if (selected) {
                selected.active = true;
                return intervals;
            }
            const iFrom = parseInt(from);
            const iTo = parseInt(to);
            const format = (t) => this.$format.date(t, '{MMM} {DD}, {HH}:{mm}');
            const f = isNaN(iFrom) ? from : format(iFrom);
            const t = isNaN(iTo) ? to : format(iTo);
            intervals.unshift({ text: (f || '') + ' to ' + (t || 'now'), query: { from, to }, active: true });
            return intervals;
        },
    },
};
</script>

<style scoped></style>
