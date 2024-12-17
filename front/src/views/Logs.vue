<template>
    <div>
        <v-card outlined class="pa-4 mb-2">
            <slot name="check">
                <Check :appId="appId" :check="check" />
            </slot>

            <div class="mt-3">
                <Led :status="data.status" />
                <template v-if="data.message">
                    <span v-html="data.message" />
                    <span v-if="data.status === 'ok'"> (<a @click="configure = true">configure</a>) </span>
                </template>
                <span v-else-if="loading">Loading...</span>
            </div>

            <v-form v-if="configured" :disabled="disabled">
                <v-select
                    :items="sources"
                    v-model="query.source"
                    @change="changeSource"
                    outlined
                    hide-details
                    dense
                    :menu-props="{ offsetY: true }"
                    class="mt-4"
                />

                <div class="subtitle-1 mt-3">Filter:</div>
                <div class="d-flex flex-wrap flex-md-nowrap align-center" style="gap: 8px">
                    <v-checkbox
                        v-for="s in severities"
                        :key="s.name"
                        :value="s.name"
                        v-model="query.severity"
                        :label="s.name"
                        :color="s.color"
                        class="ma-0 text-no-wrap text-capitalize checkbox"
                        dense
                        hide-details
                    />
                    <div class="d-flex flex-grow-1" style="gap: 4px">
                        <v-text-field
                            v-model="query.search"
                            @keydown.enter.prevent="runQuery"
                            label="Filter messages"
                            prepend-inner-icon="mdi-magnify"
                            dense
                            hide-details
                            single-line
                            outlined
                            clearable
                        >
                            <template v-if="query.hash" #prepend-inner>
                                <v-chip small label close @click:close="filterByPattern('')" close-icon="mdi-close" class="mr-2">
                                    pattern: {{ query.hash.substr(0, 7) }}
                                </v-chip>
                            </template>
                        </v-text-field>
                        <v-btn @click="runQuery" :disabled="disabled" color="primary" height="40">Query</v-btn>
                    </div>
                </div>

                <div class="subtitle-1 mt-2">View:</div>
                <div class="d-flex flex-wrap align-center" style="gap: 12px">
                    <v-btn-toggle v-model="query.view" @change="setQuery" dense>
                        <v-btn v-for="v in views" :value="v.name" @click="v.click" :disabled="v.disabled" height="40" class="text-capitalize">
                            <v-icon small>{{ v.icon }}</v-icon>
                            {{ v.name }}
                        </v-btn>
                    </v-btn-toggle>
                    <v-btn-toggle v-model="order" @change="setQuery" dense>
                        <v-btn value="desc" :disabled="disabled" height="40"><v-icon small>mdi-arrow-up-thick</v-icon>Newest first</v-btn>
                        <v-btn value="asc" :disabled="disabled" height="40"><v-icon small>mdi-arrow-down-thick</v-icon>Oldest first</v-btn>
                    </v-btn-toggle>
                    <div class="d-flex align-center" style="gap: 4px">
                        Limit:
                        <v-select
                            :items="limits"
                            v-model="query.limit"
                            @change="setQuery"
                            :disabled="disabled"
                            outlined
                            hide-details
                            dense
                            :menu-props="{ offsetY: true }"
                            style="width: 12ch"
                        />
                    </div>
                </div>
            </v-form>
            <v-progress-linear v-if="loading" indeterminate height="4" style="position: absolute; bottom: 0; left: 0" />
        </v-card>

        <div class="pt-5" style="position: relative; min-height: 50vh">
            <div v-if="!loading && loadingError" class="pa-3 text-center red--text">
                {{ loadingError }}
            </div>
            <template v-else>
                <Chart v-if="chart" :chart="chart" :selection="{}" @select="zoom" class="my-3" />

                <div v-if="query.view === 'messages'">
                    <v-simple-table v-if="entries" dense class="entries">
                        <thead>
                            <tr>
                                <th>Date</th>
                                <th>Message</th>
                            </tr>
                        </thead>
                        <tbody class="mono">
                            <tr v-for="e in entries" @click="entry = e" style="cursor: pointer">
                                <td class="text-no-wrap" style="padding-left: 1px">
                                    <div class="d-flex" style="gap: 4px">
                                        <div class="marker" :style="{ backgroundColor: e.color }" />
                                        <div>{{ e.date }}</div>
                                    </div>
                                </td>
                                <td class="text-no-wrap">{{ e.multiline ? e.message.substr(0, e.multiline) : e.message }}</td>
                            </tr>
                        </tbody>
                    </v-simple-table>
                    <div v-else-if="!loading" class="pa-3 text-center grey--text">No messages found</div>
                    <div v-if="entries && data.limit" class="text-right caption grey--text">The output is capped at {{ data.limit }} messages.</div>
                    <v-dialog v-if="entry" v-model="entry" width="80%">
                        <v-card class="pa-5 entry">
                            <div class="d-flex align-center">
                                <div class="d-flex">
                                    <v-chip label dark small :color="entry.color" class="text-uppercase mr-2">{{ entry.severity }}</v-chip>
                                    {{ entry.date }}
                                </div>
                                <v-spacer />
                                <v-btn icon @click="entry = null"><v-icon>mdi-close</v-icon></v-btn>
                            </div>

                            <div class="font-weight-medium my-3">Message</div>
                            <div class="message" :class="{ multiline: entry.multiline }">
                                {{ entry.message }}
                            </div>

                            <div class="font-weight-medium mt-4 mb-2">Attributes</div>
                            <v-simple-table dense>
                                <tbody>
                                    <tr v-for="(v, k) in entry.attributes">
                                        <td>{{ k }}</td>
                                        <td>
                                            <router-link
                                                v-if="k === 'host.name'"
                                                :to="{ name: 'overview', params: { view: 'nodes', id: v }, query: $utils.contextQuery() }"
                                            >
                                                {{ v }}
                                            </router-link>
                                            <pre v-else>{{ v }}</pre>
                                        </td>
                                    </tr>
                                </tbody>
                            </v-simple-table>
                            <v-btn
                                v-if="entry.attributes['pattern.hash']"
                                color="primary"
                                @click="filterByPattern(entry.attributes['pattern.hash'])"
                                class="mt-4"
                            >
                                Show similar messages
                            </v-btn>
                            <v-btn
                                v-if="entry.attributes['trace.id']"
                                color="primary"
                                :to="{
                                    params: { report: 'Tracing' },
                                    query: { query: undefined, trace: 'otel:' + entry.attributes['trace.id'] + ':-:-:' },
                                }"
                                class="mt-4"
                            >
                                Show the trace
                            </v-btn>
                        </v-card>
                    </v-dialog>
                </div>

                <div v-if="query.view === 'patterns'">
                    <div v-if="patterns" class="patterns">
                        <div v-for="p in patterns" class="pattern" @click="pattern = p">
                            <div class="sample">{{ p.sample }}</div>
                            <div class="line">
                                <v-sparkline v-if="p.messages" :value="p.messages" smooth height="30" fill :color="p.color" padding="4" />
                            </div>
                            <div class="percent">{{ p.percent }}</div>
                        </div>
                    </div>
                    <div v-else-if="!loading" class="pa-3 text-center grey--text">No patterns found</div>
                    <v-dialog v-if="pattern" v-model="pattern" width="80%">
                        <v-card tile class="pa-5">
                            <div class="d-flex align-center">
                                <div class="d-flex">
                                    <v-chip label dark small :color="pattern.color" class="text-uppercase mr-2">{{ pattern.severity }}</v-chip>
                                    {{ pattern.sum }} events
                                </div>
                                <v-spacer />
                                <v-btn icon @click="pattern = null"><v-icon>mdi-close</v-icon></v-btn>
                            </div>
                            <Chart v-if="pattern.chart" :chart="pattern.chart" />
                            <div class="font-weight-medium my-3">Sample</div>
                            <div class="message" :class="{ multiline: pattern.multiline }">
                                {{ pattern.sample }}
                            </div>
                            <v-btn v-if="configured" color="primary" @click="filterByPattern(pattern.hash)" class="mt-4"> Show messages </v-btn>
                        </v-card>
                    </v-dialog>
                </div>
            </template>
        </div>

        <v-dialog v-model="configure" max-width="800">
            <v-card class="pa-5">
                <div class="d-flex align-center font-weight-medium mb-4">
                    Link "{{ $utils.appId(appId).name }}" with a service
                    <v-spacer />
                    <v-btn icon @click="configure = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>

                <div class="subtitle-1">Choose a corresponding OpenTelemetry service:</div>
                <v-select v-model="form.service" :items="services" outlined dense hide-details :menu-props="{ offsetY: true }" clearable />

                <div class="grey--text my-4">
                    To configure an application to send logs follow the
                    <a href="https://coroot.com/docs/coroot/logs" target="_blank">documentation</a>.
                </div>

                <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="my-3">
                    {{ error }}
                </v-alert>
                <v-alert v-if="message" color="green" outlined text class="my-3">
                    {{ message }}
                </v-alert>
                <v-btn block color="primary" @click="save" :loading="saving" :disabled="!changed" class="mt-5">Save</v-btn>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
import Led from '../components/Led.vue';
import Chart from '../components/Chart.vue';
import Check from '../components/Check.vue';
import { palette } from '../utils/colors';

const severity = (s) => {
    s = s.toLowerCase();
    if (s.startsWith('crit')) return { num: 5, color: 'black' };
    if (s.startsWith('err')) return { num: 4, color: 'red-darken1' };
    if (s.startsWith('warn')) return { num: 3, color: 'orange-lighten1' };
    if (s.startsWith('info')) return { num: 2, color: 'blue-lighten2' };
    if (s.startsWith('debug')) return { num: 1, color: 'green-lighten1' };
    return { num: 0, color: 'grey-lighten1' };
};

export default {
    components: { Led, Chart, Check },
    props: {
        appId: String,
        check: Object,
    },

    data() {
        return {
            loading: false,
            loadingError: '',
            data: {},
            query: {},
            order: '',

            configure: false,
            form: {
                service: null,
            },
            saved: '',
            saving: false,
            error: '',
            message: '',

            entry: null,
            pattern: null,
        };
    },

    computed: {
        configured() {
            return this.data.status !== 'unknown';
        },
        sources() {
            return (this.data.sources || []).map((s) => {
                return {
                    value: s,
                    text: s === 'otel' ? 'OpenTelemetry' : 'Container logs',
                };
            });
        },
        services() {
            return this.data.services || [];
        },
        views() {
            const views = this.data.views || [];
            const res = [
                { name: 'messages', icon: 'mdi-format-list-bulleted', click: () => {} },
                { name: 'patterns', icon: 'mdi-creation', click: () => this.filterByPattern('') },
            ];
            res.forEach((v) => {
                v.disabled = views.indexOf(v.name) < 0;
            });
            return res;
        },
        severities() {
            if (!this.data.severities) {
                return [];
            }
            const res = this.data.severities.map((s) => {
                const sev = severity(s);
                return {
                    name: s,
                    num: sev.num,
                    color: palette.get(sev.color),
                };
            });
            res.sort((s1, s2) => s1.num - s2.num);
            return res;
        },
        chart() {
            const ch = this.data.chart;
            if (!ch) {
                return null;
            }
            if (!ch.series) {
                return ch;
            }
            ch.series.forEach((s) => {
                const sev = severity(s.name);
                s.num = sev.num;
                s.color = sev.color;
            });
            ch.series.sort((s1, s2) => s1.num - s2.num);
            ch.sorted = true;
            return ch;
        },
        entries() {
            if (!this.data.entries) {
                return null;
            }
            const res = this.data.entries.map((e) => {
                const newline = e.message.indexOf('\n');
                return {
                    severity: e.severity,
                    timestamp: e.timestamp,
                    color: palette.get(severity(e.severity).color),
                    date: this.$format.date(e.timestamp, '{MMM} {DD} {HH}:{mm}:{ss}'),
                    message: e.message,
                    attributes: e.attributes,
                    multiline: newline > 0 ? newline : 0,
                };
            });
            if (this.order === 'asc') {
                res.sort((e1, e2) => e1.timestamp - e2.timestamp);
            } else {
                res.sort((e1, e2) => e2.timestamp - e1.timestamp);
            }
            return res;
        },
        patterns() {
            if (!this.data.patterns) {
                return null;
            }
            let total = this.data.patterns.reduce((t, p) => t + p.sum, 0);
            return this.data.patterns.map((p) => {
                const percent = (p.sum * 100) / total;
                const newline = p.sample.indexOf('\n');
                let messages = null;
                if (p.chart && p.chart.series && p.chart.series[0]) {
                    p.chart.series[0].color = (severity(p.severity) || {}).color;
                    messages = p.chart.series[0].data.map((v) => (v === null ? 0 : v));
                }
                return {
                    severity: p.severity,
                    color: palette.get(severity(p.severity).color),
                    sample: p.sample,
                    multiline: newline > 0 ? newline : 0,
                    messages: messages,
                    sum: p.sum,
                    percent: (percent < 1 ? '<1' : Math.trunc(percent)) + '%',
                    hash: p.hash,
                    chart: p.chart,
                };
            });
        },
        limits() {
            return [10, 20, 50, 100, 1000];
        },
        disabled() {
            return this.loading || this.query.view !== 'messages';
        },
        changed() {
            return !!this.form && this.saved !== JSON.stringify(this.form);
        },
    },

    mounted() {
        this.getQuery();
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        '$route.query'(curr, prev) {
            this.getQuery();
            if (curr.query !== prev.query) {
                this.get();
            }
        },
    },

    methods: {
        getQuery() {
            const query = this.$route.query;
            let q = {};
            try {
                q = JSON.parse(query.query || '{}');
            } catch {
                //
            }
            let severity = q.severity || [];
            if (!severity.length) {
                severity = this.data.severities || [];
            }
            this.query = {
                source: q.source || '',
                view: q.view || 'messages',
                search: q.search || '',
                hash: q.hash || '',
                severity,
                limit: q.limit || 100,
            };
            this.order = query.order || 'desc';
        },
        setQuery() {
            if (this.query.view === 'patterns') {
                this.query.severity = this.data.severities || [];
                this.query.search = '';
                this.query.hash = '';
                this.order = '';
            }
            const query = {
                query: JSON.stringify(this.query),
                view: this.view,
                order: this.order,
            };
            this.$router.push({ query: { ...this.$route.query, ...query } }).catch((err) => err);
        },
        runQuery() {
            const q = this.$route.query.query;
            this.setQuery();
            if (this.$route.query.query === q) {
                this.get();
            }
        },
        changeSource(s) {
            this.data.severities = [];
            this.query.source = s;
            this.query.severity = [];
            this.query.search = '';
            this.query.hash = '';
            this.setQuery();
        },
        filterByPattern(hash) {
            this.query.view = 'messages';
            this.pattern = null;
            this.query.hash = hash;
            this.entry = null;
            this.setQuery();
        },
        zoom(s) {
            const { from, to } = s.selection;
            const query = { ...this.$route.query, from, to };
            this.$router.push({ query }).catch((err) => err);
        },
        get() {
            this.loading = true;
            this.loadingError = '';
            this.data.chart = null;
            this.data.entries = null;
            this.data.patterns = null;
            this.$api.getLogs(this.appId, this.$route.query.query, (data, error) => {
                this.loading = false;
                const errMsg = 'Failed to load logs';
                if (error || data.status === 'warning') {
                    this.loadingError = error || data.message;
                    this.data.status = 'warning';
                    this.data.message = errMsg;
                    return;
                }
                this.data = data;
                this.form.service = this.data.service;
                this.saved = JSON.stringify(this.form);
                this.query.source = this.data.source;
                this.query.view = this.data.view;
                this.query.severity = this.data.severity;
            });
        },
        save() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveLogsSettings(this.appId, this.form, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                    this.configure = false;
                }, 1000);
                this.get();
            });
        },
    },
};
</script>

<style scoped>
.mono {
    font-family: monospace, monospace;
}
.marker {
    height: 20px;
    width: 4px;
    filter: brightness(var(--brightness));
}
.checkbox:deep(.v-input--selection-controls__input) {
    margin-left: -5px;
    margin-right: 0 !important;
}

.pattern {
    display: flex;
    align-items: flex-end;
    margin-bottom: 8px;
    cursor: pointer;
    background-color: var(--background-color-hi);
    padding: 4px 8px;
    border-radius: 2px;
    filter: brightness(var(--brightness));
}
.pattern .sample {
    font-size: 0.8rem;
    flex-grow: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-height: 4.5rem;
}
.pattern .line {
    flex-grow: 0;
    flex-basis: 30%;
    max-width: 30%;
    flex-shrink: 0;
}
.pattern .percent {
    flex-grow: 0;
    flex-basis: 2rem;
    max-width: 2rem;
    flex-shrink: 0;
    font-size: 0.75rem;
    text-align: right;
}
.pattern .percent:deep(span) {
    font-size: 0.65rem;
}

.entry:deep(tr:hover) {
    background-color: unset !important;
}

.message {
    font-family: monospace, monospace;
    font-size: 14px;
    background-color: var(--background-color-hi);
    filter: brightness(var(--brightness));
    border-radius: 3px;
    max-height: 50vh;
    padding: 8px;
    overflow: auto;
}
.message.multiline {
    white-space: pre;
}
</style>
