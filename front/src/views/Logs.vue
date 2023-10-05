<template>
<div>
    <v-card outlined class="pa-4 mb-2">
        <Check :appId="appId" :check="check" />

        <div class="mt-3">
            <Led :status="data.status" />
            <template v-if="data.message">
                <span v-html="data.message" />
                <span v-if="data.status !== 'warning'">
                    (<a @click="configure = true">configure</a>)
                </span>
            </template>
            <span v-else-if="loading">Loading...</span>
            <v-progress-circular v-if="loading" indeterminate size="16" width="2" color="green" />
        </div>

        <v-form :disabled="loading">
            <v-select :items="sources" v-model="query.source" @change="setQuery" outlined hide-details dense :menu-props="{offsetY: true}" class="mt-4" />

            <div class="subtitle-1 mt-3">Filter: </div>
            <div class="d-flex flex-wrap align-center" style="margin-left: -8px">
                <v-checkbox v-for="(s, name) in severities" :key="name" :value="name" v-model="query.severity" @change="setQuery" :label="name" :color="color(s.color)" class="ma-0 mx-1 text-no-wrap text-capitalize checkbox" dense hide-details />
            </div>

            <v-text-field v-model="query.search" @keydown.enter.prevent="setQuery" dense hide-details prepend-inner-icon="mdi-magnify" label="Filter messages" single-line outlined class="mt-2">
                <template v-if="query.hash && query.hash.length" #prepend-inner>
                    <v-chip small label close @click:close="filterByPattern(null)" close-icon="mdi-close" class="mr-2">
                        pattern: {{query.hash[0].substr(0, 7)}}
                    </v-chip>
                </template>
                <template #append>
                    <v-btn @click="setQuery" small color="primary" style="margin-top: -2px; margin-right: -5px">Search</v-btn>
                </template>
            </v-text-field>
        </v-form>

        <div class="subtitle-1 mt-2">View: </div>
        <div class="d-flex" style="gap: 12px">
            <v-btn-toggle v-model="view" @change="setQuery" dense>
                <v-btn value="list"><v-icon small>mdi-format-list-bulleted</v-icon>List</v-btn>
                <v-btn value="patterns" @click="filterByPattern(null)"><v-icon small>mdi-creation</v-icon>Patterns</v-btn>
            </v-btn-toggle>
            <v-btn-toggle v-model="order" @change="setQuery" dense>
                <v-btn value="desc" :disabled="view !== 'list'"><v-icon small>mdi-arrow-up-thick</v-icon>Newest first</v-btn>
                <v-btn value="asc" :disabled="view !== 'list'"><v-icon small>mdi-arrow-down-thick</v-icon>Oldest first</v-btn>
            </v-btn-toggle>
        </div>

        <div class="grey--text mt-5">
            <v-icon size="20" style="vertical-align: baseline">mdi-lightbulb-on-outline</v-icon>
            Select a chart area to zoom in
        </div>
    </v-card>

    <div class="pt-5" style="position: relative">
        <v-progress-linear v-if="loading" indeterminate color="green" height="4" style="position: absolute; top: 0" />

        <div v-if="!loading && loadingError" class="pa-3 text-center red--text">
            {{loadingError}}
        </div>

        <Chart v-if="chart" :chart="chart" :selection="{}" @select="zoom" />

        <div v-if="view === 'list'" class="mt-5">
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
                            <div class="marker" :style="{backgroundColor: e.color}" />
                            <div>{{e.date}}</div>
                        </div>
                    </td>
                    <td class="text-no-wrap">{{e.multiline ? e.message.substr(0, e.multiline) : e.message}}</td>
                </tr>
                </tbody>
            </v-simple-table>
            <div v-else-if="!loading" class="pa-3 text-center grey--text">
                No messages found
            </div>
            <div v-if="entries && data.limit" class="text-right caption grey--text">
                The output is capped at {{data.limit}} messages.
            </div>
        </div>

        <div v-if="view === 'patterns' && patterns" class="patterns mt-5">
            <div v-for="p in patterns" class="pattern" @click.stop="filterByPattern(p)">
                <div class="sample preview" v-html="p.sample" />
                <div class="line">
                    <v-sparkline :value="p.messages" smooth height="30" fill :color="p.color" padding="4" />
                </div>
                <div class="percent">{{p.percent}}</div>
            </div>
        </div>

    </div>

    <v-dialog v-if="entry" v-model="entry" width="80%">
        <v-card class="pa-5 details">
            <div class="d-flex align-center">
                <div class="d-flex">
                    <v-chip label dark small :color="entry.color" class="text-uppercase mr-2">{{entry.severity}}</v-chip>
                    {{entry.date}}
                </div>
                <v-spacer />
                <v-btn icon @click="entry = null"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <div class="font-weight-medium mt-4 mb-2">Message</div>
            <div class="message" :class="{multiline: entry.multiline}" v-html="entry.message" />

            <div class="font-weight-medium mt-4 mb-2">Attributes</div>
            <v-simple-table dense>
                <tbody>
                <tr v-for="(v, k) in entry.attributes">
                    <td>{{k}}</td>
                    <td><pre>{{v}}</pre></td>
                </tr>
                </tbody>
            </v-simple-table>
        </v-card>
    </v-dialog>

    <v-dialog v-model="configure" max-width="800">
        <v-card class="pa-5">
            <div class="d-flex align-center font-weight-medium mb-4">
                Link "{{ $utils.appId(appId).name }}" with a service
                <v-spacer />
                <v-btn icon @click="configure = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <div class="subtitle-1">Choose a corresponding OpenTelemetry service:</div>
            <v-select v-model="form.service" :items="services" outlined dense hide-details :menu-props="{offsetY: true}" clearable />

            <div class="grey--text my-4">
                To configure an application to send logs follow the <a href="https://coroot.com/docs/coroot-community-edition/logs" target="_blank">documentation</a>.
            </div>

            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text class="my-3">
                {{error}}
            </v-alert>
            <v-alert v-if="message" color="green" outlined text class="my-3">
                {{message}}
            </v-alert>
            <v-btn block color="primary" @click="save" :loading="saving" :disabled="!changed" class="mt-5">Save</v-btn>
        </v-card>
    </v-dialog>
</div>
</template>

<script>
import Check from "./Check.vue";
import Chart from "../components/Chart.vue";
import { palette } from "../utils/colors";
import Led from "@/components/Led.vue";

const severities = {
    unknown: {color: 'grey-lighten1'},
    debug: {color: 'green-lighten1'},
    info: {color: 'blue-lighten2'},
    warning: {color: 'orange-lighten1'},
    error: {color: 'red-darken1'},
    critical: {color: 'black'},
};

export default {
    components: {Led, Chart, Check},
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
            view: '',
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
        }
    },

    computed: {
        sources() {
            return (this.data.sources || []).map(s => ({
                text: s.name,
                value: s.type,
            }));
        },
        services() {
            return (this.data.services || []).map(a => a.name);
        },
        severities() {
            return severities;
        },
        chart() {
            if (!this.data.chart) {
                return null;
            }
            this.data.chart.series.forEach(s => {
                s.color = (severities[s.name] || {}).color;
            });
            return this.data.chart;
        },
        entries() {
            if (!this.data.entries) {
                return null;
            }
            const res = this.data.entries.map(e => {
                const i = e.message.indexOf('\n');
                return {
                    severity: e.severity,
                    timestamp: e.timestamp,
                    color: palette.get(severities[e.severity].color),
                    date: this.$format.date(e.timestamp, '{MMM} {DD} {HH}:{mm}:{ss}'),
                    message: e.message,
                    attributes: e.attributes,
                    multiline: i > 0 ? i : 0,
                }
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
            return this.data.patterns.map(p => {
                const percent = p.sum * 100 / total;
                return {
                    sample: p.sample,
                    color: palette.get(severities[p.severity].color),
                    messages: p.messages.map((v) => v === null ? 0 : v),
                    sum: p.sum,
                    percent: (percent < 1 ? '<1' : Math.trunc(percent)) + '%',
                    hash: p.hash,
                };
            });
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
        }
    },

    methods: {
        color(c) {
            return palette.get(c);
        },
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
                severity = Object.keys(severities);
            }
            this.query = {
                source: q.source || '',
                search: q.search || '',
                hash: q.hash || [],
                severity,
            };
            this.view = query.view || 'list';
            this.order = query.order || 'desc';
        },
        setQuery() {
            const query = {
                query: JSON.stringify(this.query),
                view: this.view,
                order: this.order,
            };
            this.$router.push({query: {...this.$route.query, ...query}}).catch(err => err);
        },
        zoom(s) {
            const {from, to} = s.selection;
            const query = {...this.$route.query, from, to};
            this.$router.push({query}).catch(err => err);
        },
        filterByPattern(p) {
            this.view = 'list';
            this.query.hash = p ? p.hash : [];
            this.setQuery();
        },
        get() {
            this.loading = true;
            this.loadingError = '';
            this.$api.getLogs(this.appId, this.$route.query.query, (data, error) => {
                this.loading = false;
                const errMsg = 'Failed to load traces';
                if (error) {
                    this.loadingError = error;
                    this.data.status = 'warning';
                    this.data.message = errMsg;
                    this.data.logs = null;
                    this.data.patterns = null;
                    return;
                }
                this.data = data;
                if (this.data.status === 'warning') {
                    this.loadingError = this.data.message;
                    this.data.message = errMsg;
                    return;
                }
                const service = (this.data.services || []).find(s => s.linked);
                this.form.service = service ? service.name : null;
                this.saved = JSON.stringify(this.form);
                const source = (this.data.sources || []).find((s) => s.selected);
                if (source) {
                    this.query.source = source.type;
                }
            })
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
}
</script>

<style scoped>
.mono {
    font-family: monospace, monospace;
}
.marker {
    height: 20px;
    width: 4px;
}
.checkbox:deep(.v-input--selection-controls__input) {
    margin-right: 0 !important;
}

.pattern {
    display: flex;
    align-items: flex-end;
    margin-bottom: 8px;
    cursor: pointer;
    background-color: #EEEEEE;
    padding: 4px 8px;
    border-radius: 2px;
}
.pattern:hover {
    background-color: #E0E0E0;
}
.pattern .sample {
    font-size: 0.8rem;
}
.pattern .sample.preview {
    flex-grow: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-height: 4.5rem;
}
.pattern .sample.details {
    background-color: #EEEEEE;
    overflow: auto;
    border-radius: 3px;
}
.pattern .sample.details.multiline {
    white-space: pre;
    max-height: 50vh;
}
.pattern .sample >>> mark {
    background-color: unset;
    color: black;
    font-weight: bold;
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

.details .message {
    font-family: monospace, monospace;
    font-size: 14px;
    background-color: #EEEEEE;
    border-radius: 3px;
    max-height: 50vh;
    padding: 8px;
    overflow: auto;
}
.details .message.multiline {
    white-space: pre;
}
.details:deep(tr:hover) {
    background-color: unset !important;
}
</style>