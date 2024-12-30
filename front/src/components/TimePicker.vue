<template>
    <v-menu v-model="menu" :close-on-content-click="false" :left="$vuetify.breakpoint.xsOnly" offset-y attach=".v-app-bar">
        <template #activator="{ on, attrs }">
            <v-btn v-on="on" plain outlined height="40" class="px-2">
                <v-icon>mdi-clock-outline</v-icon>
                <span v-if="!small" class="ml-2">{{ intervals.find((i) => i.active).text }}</span>
                <v-icon v-if="!small" small class="ml-2"> mdi-chevron-{{ attrs['aria-expanded'] === 'true' ? 'up' : 'down' }} </v-icon>
            </v-btn>
        </template>
        <v-list dense dark>
            <v-list-item v-for="i in intervals" :key="i.text" :to="{ query: i.query }" @click="quick" exact>
                {{ i.text }}
            </v-list-item>
            <v-divider />
            <v-form class="pa-2" @submit.prevent="custom">
                <div class="mx-2 mb-2">Custom range:</div>
                <v-text-field
                    v-model="from"
                    outlined
                    dense
                    label="From"
                    class="mb-2"
                    hide-details
                    append-icon="mdi-calendar-month-outline"
                    @click="open"
                    @click:append="picker = !picker"
                />
                <v-text-field
                    v-model="to"
                    outlined
                    dense
                    label="To"
                    class="mb-3"
                    hide-details
                    append-icon="mdi-calendar-month-outline"
                    @click:append="picker = !picker"
                />
                <div v-if="!picker" class="d-flex">
                    <v-spacer />
                    <v-btn small color="primary" :disabled="!valid" type="submit" @click="custom">Apply</v-btn>
                </div>
            </v-form>
            <v-date-picker v-if="picker" v-model="dates" @change="change" no-title range dark color="currentColor" />
        </v-list>
    </v-menu>
</template>

<script>
export default {
    props: {
        small: Boolean,
    },

    data() {
        return {
            menu: false,
            picker: false,
            dates: [],
            from: '',
            to: '',
        };
    },

    mounted() {
        const fmt = '{YYYY}-{MM}-{DD} {HH}:{mm}';
        const f = parseInt(this.$route.query.from);
        const t = parseInt(this.$route.query.to);
        this.from = !isNaN(f) ? this.$format.date(f, fmt) : '';
        this.to = !isNaN(t) ? this.$format.date(t, fmt) : '';
        this.dates = [this.from.split(' ')[0], this.to.split(' ')[0]];
    },

    watch: {
        menu(v) {
            if (!v) {
                this.picker = false;
            }
        },
    },

    methods: {
        quick() {
            this.menu = false;
            this.from = '';
            this.to = '';
            this.dates = [];
            this.picker = false;
        },
        custom() {
            this.menu = false;
            const from = new Date(this.from).getTime();
            const to = new Date(this.to).getTime();
            this.$router.push({ query: { from, to } }).catch((err) => err);
        },
        open() {
            if (!this.from && !this.to) {
                this.picker = true;
            }
        },
        change() {
            this.from = this.dates[0] + ' 00:00';
            this.to = this.dates[1] + ' 23:59';
            this.picker = false;
        },
    },

    computed: {
        valid() {
            return !!new Date(this.from).valueOf() && !!new Date(this.to).valueOf();
        },
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

<style scoped>
*:deep(.v-picker__body) {
    background-color: var(--background-dark);
    border-radius: 0 !important;
    border-right: 1px solid var(--border-dark);
}
</style>
