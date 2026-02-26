<template>
    <v-menu v-model="menu" :close-on-content-click="false" left offset-y attach=".v-app-bar">
        <template #activator="{ on, attrs }">
            <v-btn v-on="on" plain outlined height="40" class="px-2">
                <v-icon>mdi-clock-outline</v-icon>
                <span v-if="!small" class="ml-2">{{ intervals.find((i) => i.active).text }}</span>
                <v-icon v-if="!small" small class="ml-2"> mdi-chevron-{{ attrs['aria-expanded'] === 'true' ? 'up' : 'down' }} </v-icon>
            </v-btn>
        </template>
        <v-list dense class="list">
            <v-list-item v-for="i in intervals" :key="i.text" @click="quick(i)" :input-value="i.active">
                <v-list-item-content>
                    {{ i.text }}
                </v-list-item-content>
            </v-list-item>
            <v-divider />
            <v-form class="pa-2" @submit.prevent="apply">
                <div class="mx-2 mb-2">Custom range:</div>
                <v-text-field
                    v-model="from"
                    outlined
                    dense
                    label="From"
                    class="mb-2"
                    hide-details
                    append-icon="mdi-calendar-month-outline"
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
                    <v-btn small color="primary" :disabled="!valid" type="submit" @click="apply">Apply</v-btn>
                </div>
            </v-form>
            <v-date-picker v-if="picker" v-model="dates" @change="change" no-title range dark color="currentColor" class="picker" />
        </v-list>
    </v-menu>
</template>

<script>
function isRelative(t) {
    return /^now(-\d+[mhdw])?$/.test(t);
}

function isAbsolute(t) {
    return !t.startsWith('now') && !!new Date(t).valueOf();
}

function isValid(t) {
    return isRelative(t) || isAbsolute(t);
}

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

    watch: {
        menu(v) {
            if (!v) {
                this.picker = false;
                return;
            }
            const fmt = '{YYYY}-{MM}-{DD} {HH}:{mm}';
            const from = this.$route.query.from || 'now-1h';
            const to = this.$route.query.to || 'now';
            const iFrom = parseInt(from);
            const iTo = parseInt(this.$route.query.to);
            this.from = isNaN(iFrom) ? from : this.$format.date(iFrom, fmt);
            this.to = isNaN(iTo) ? to : this.$format.date(iTo, fmt);
            this.dates = isAbsolute(this.from) && isAbsolute(this.to) ? [this.from.split(' ')[0], this.to.split(' ')[0]] : [];
        },
    },

    methods: {
        quick(interval) {
            // eslint-disable-next-line no-unused-vars
            const { from, to, incident, alert, ...query } = this.$route.query;
            this.$router.push({ query: { ...query, ...interval.query } }).catch((err) => err);
            this.menu = false;
            this.from = this.$route.query.from || 'now-1h';
            this.to = this.$route.query.to || 'now';
            this.dates = [];
            this.picker = false;
        },
        apply() {
            this.menu = false;
            const from = isRelative(this.from) ? this.from : new Date(this.from).getTime();
            const to = isRelative(this.to) ? this.to : new Date(this.to).getTime();
            // eslint-disable-next-line no-unused-vars
            const { incident, alert, ...query } = this.$route.query;
            this.$router.push({ query: { ...query, from, to } }).catch((err) => err);
        },
        change() {
            this.from = this.dates[0] + ' 00:00';
            this.to = this.dates[1] + ' 23:59';
            this.picker = false;
        },
    },

    computed: {
        valid() {
            return isValid(this.from) && isValid(this.to) && this.from !== this.to;
        },
        intervals() {
            const intervals = [
                { text: 'last 5 minutes', query: { from: 'now-5m' } },
                { text: 'last 15 minutes', query: { from: 'now-15m' } },
                { text: 'last 30 minutes', query: { from: 'now-30m' } },
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
            const alert = this.$route.query.alert;
            if (alert) {
                intervals.unshift({ text: 'alert: ' + alert, query: { alert }, active: true });
                return intervals;
            }
            const from = this.$route.query.from;
            const to = this.$route.query.to === 'now' ? undefined : this.$route.query.to;
            const selected = intervals.find((i) => i.query.from === from && i.query.to === to);
            if (selected) {
                selected.active = true;
                return intervals;
            }
            const fmt = '{MMM} {DD}, {HH}:{mm}';
            const iFrom = parseInt(from);
            const iTo = parseInt(to);
            const f = isNaN(iFrom) ? from : this.$format.date(iFrom, fmt);
            const t = isNaN(iTo) ? to : this.$format.date(iTo, fmt);
            intervals.unshift({ text: (f || '') + ' to ' + (t || 'now'), query: { from, to }, active: true });
            return intervals;
        },
    },
};
</script>

<style scoped>
.list:deep(.v-list-item) {
    min-height: 36px;
}
.picker:deep(.v-picker__body) {
    background-color: var(--background-dark);
    border-radius: 0 !important;
    border-right: 1px solid var(--border-dark);
}
</style>
