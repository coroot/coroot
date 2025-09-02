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

            <v-form v-if="configured">
                <v-select :items="sources" v-model="query.source" outlined hide-details dense :menu-props="{ offsetY: true }" class="mt-4" />

                <div class="subtitle-1 mt-3">Query:</div>
                <div class="d-flex flex-wrap flex-md-nowrap" style="gap: 8px">
                    <QueryBuilder
                        v-model="query.filters"
                        :loading="qb.loading"
                        :disabled="query.view !== 'messages'"
                        :items="qb.items"
                        :error="qb.error"
                        @get="qbGet"
                        class="flex-grow-1"
                    />
                    <v-btn @click="get" :disabled="disabled" color="primary" height="40">Show logs</v-btn>
                    <v-btn @click="toggleLive" :color="live ? 'green' : ''" outlined height="40" :disabled="!configured || query.view !== 'messages'">
                        <v-icon small class="mr-1">{{ live ? 'mdi-pause' : 'mdi-play' }}</v-icon>
                        {{ live ? 'Live: ON' : 'Live: OFF' }}
                    </v-btn>
                </div>

                <v-btn-toggle v-model="query.view" @change="setQuery" mandatory dense class="mt-2">
                    <v-btn value="messages" :disabled="false" height="40"><v-icon small>mdi-format-list-bulleted</v-icon>Messages</v-btn>
                    <v-btn value="patterns" :disabled="query.source !== 'agent'" height="40"><v-icon small>mdi-creation</v-icon>Patterns</v-btn>
                </v-btn-toggle>
            </v-form>
            <v-progress-linear v-if="loading" indeterminate height="4" style="position: absolute; bottom: 0; left: 0" />
        </v-card>

        <div class="pt-5" style="position: relative; min-height: 50vh">
            <div v-if="!loading && loadingError" class="pa-3 text-center red--text">
                {{ loadingError }}
            </div>
            <template v-else>
                <Chart v-if="data.chart" :chart="data.chart" :selection="{}" @select="zoom" class="my-3" />

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
                                <td class="text-no-wrap px-2 pl-0">
                                    <div class="d-flex gap-1">
                                        <div class="marker" :style="{ backgroundColor: e.color }" />
                                        <div>{{ e.date }}</div>
                                    </div>
                                </td>
                                <td class="text-no-wrap">{{ e.multiline ? e.message.substr(0, e.multiline) : e.message }}</td>
                            </tr>
                        </tbody>
                    </v-simple-table>
                    <div v-else-if="!loading" class="pa-3 text-center grey--text">No messages found</div>
                    <div v-if="entries && entries.length === query.limit" class="text-right caption grey--text mt-1">
                        The output is capped at
                        <InlineSelect v-model="query.limit" :items="limits" />
                        messages.
                    </div>
                    <LogEntry v-if="entry" v-model="entry" @filter="qbAdd" :appId="appId" />
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
                    <LogPattern v-if="pattern" v-model="pattern" :messages="configured" @filter="qbAdd" />
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
import { palette } from '../utils/colors';
import Led from '../components/Led.vue';
import Chart from '../components/Chart.vue';
import Check from '../components/Check.vue';
import QueryBuilder from '@/components/QueryBuilder.vue';
import LogEntry from '@/components/LogEntry.vue';
import LogPattern from '@/components/LogPattern.vue';
import InlineSelect from '@/components/InlineSelect.vue';

export default {
    props: {
        appId: String,
        check: Object,
    },

    components: { InlineSelect, Led, Chart, Check, QueryBuilder, LogPattern, LogEntry },

    data() {
        let q = {};
        try {
            q = JSON.parse(this.$route.query.query || '{}');
        } catch {
            //
        }

        return {
            loading: false,
            loadingError: '',
            data: {},
            live: false,
            liveInterval: null,
            query: {
                source: q.source || 'agent',
                view: q.view || 'messages',
                filters: q.filters || [],
                limit: q.limit || 100,
            },
            limits: [10, 20, 50, 100, 1000],
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

            qb: {
                loading: false,
                error: '',
                items: [],
            },
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
        entries() {
            if (!this.data.entries) {
                return null;
            }
            return this.data.entries.map((e) => {
                const newline = e.message.indexOf('\n');
                return {
                    ...e,
                    color: palette.get(e.color),
                    date: this.$format.date(e.timestamp, '{MMM} {DD} {HH}:{mm}:{ss}'),
                    multiline: newline > 0 ? newline : 0,
                };
            });
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
                    messages = p.chart.series[0].data.map((v) => (v === null ? 0 : v));
                }
                return {
                    ...p,
                    color: palette.get(p.color),
                    multiline: newline > 0 ? newline : 0,
                    messages: messages,
                    percent: (percent < 1 ? '<1' : Math.trunc(percent)) + '%',
                };
            });
        },
        disabled() {
            return this.loading || this.query.view !== 'messages';
        },
        changed() {
            return !!this.form && this.saved !== JSON.stringify(this.form);
        },
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        query: {
            handler(curr, prev) {
                this.setQuery(curr.view !== prev.view);
                this.get();
            },
            deep: true,
        },
    },

    methods: {
        setQuery(push) {
            const to = { query: { ...this.$route.query, query: JSON.stringify(this.query) } };
            if (push) {
                this.$router.push(to).catch((err) => err);
            } else {
                this.$router.replace(to).catch((err) => err);
            }
        },
        zoom(s) {
            const { from, to } = s.selection;
            const query = { ...this.$route.query, from, to };
            this.$router.push({ query }).catch((err) => err);
        },
        qbAdd(name, op, value) {
            this.query.view = 'messages';
            this.entry = null;
            this.pattern = null;
            this.query.filters.push({ name, op, value });
            this.setQuery();
        },
        qbGet(what, name) {
            this.qb.items = [];
            if (what === 'op') {
                switch (name) {
                    case 'Severity':
                        this.qb.items = ['=', '!='];
                        break;
                    case 'Message':
                        this.qb.items = ['contains'];
                        break;
                    case 'pattern.hash':
                        this.qb.items = ['='];
                        break;
                    default:
                        this.qb.items = ['=', '!=', '~', '!~'];
                }
                return;
            }
            this.qb.loading = true;
            this.qb.error = '';
            const query = JSON.stringify({ ...this.query, suggest: what === 'value' ? name : '' });
            this.$api.getLogs(this.appId, query, (data, error) => {
                this.qb.loading = false;
                if (error || data.status === 'warning') {
                    this.qb.error = error || data.message;
                    return;
                }
                this.qb.items = data.suggest || [];
            });
        },
        get() {
            this.loading = true;
            this.loadingError = '';
            this.data.chart = null;
            this.data.entries = null;
            this.data.patterns = null;
            if (this.live) { this.stopLive(); }
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
                if (this.query.view === 'messages' && this.live && this.configured) {
                    this.startLive();
                }
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
        buildLogsQuery() {
            return JSON.stringify({
                source: this.query.source,
                filters: this.query.filters,
                limit: this.query.limit,
                view: 'messages'
            });
        },
        startLive() {
            if (this.liveInterval || !this.configured) return;
            
            console.log('Starting live logs polling mode');
            this.live = true;
            
            let lastTimestamp = Date.now() * 1000; // Convert to microseconds for ClickHouse
            
            const pollLogs = () => {
                if (!this.live) return;
                
                const now = Date.now() * 1000; // Current time in microseconds
                
                // Adjust time range: from last timestamp + 1μs to now
                const timeQuery = {
                    from: Math.floor(lastTimestamp / 1000) + 1, // Convert back to milliseconds and add 1ms
                    to: Math.floor(now / 1000)
                };
                
                const queryParams = new URLSearchParams({
                    ...this.$route.query,
                    ...timeQuery,
                    query: this.buildLogsQuery()
                });
                
                this.$api.getLogs(this.appId, queryParams.toString(), (data, error) => {
                    if (error) {
                        console.error('Live logs polling error:', error);
                        return;
                    }
                    
                    if (data.status === 'warning') {
                        console.warn('Live logs warning:', data.message);
                        return;
                    }
                    
                    // Process new entries
                    if (data.entries && data.entries.length > 0) {
                        const newEntries = data.entries.map((e) => {
                            const newline = e.message ? e.message.indexOf('\n') : -1;
                            return {
                                ...e,
                                color: palette.get(e.color),
                                date: this.$format.date(e.timestamp, '{MMM} {DD} {HH}:{mm}:{ss}'),
                                multiline: newline > 0 ? newline : 0,
                            };
                        });
                        
                        if (!this.data.entries) {
                            this.$set(this.data, 'entries', []);
                        }
                        
                        // Add new entries to the beginning (newest first)
                        this.data.entries.unshift(...newEntries.reverse());
                        
                        // Update last timestamp to the newest entry
                        const timestamps = data.entries.map(e => e.timestamp);
                        if (timestamps.length > 0) {
                            lastTimestamp = Math.max(...timestamps.map(t => new Date(t).getTime() * 1000));
                        }
                        
                        // Keep limit of entries on screen
                        if (this.data.entries.length > this.query.limit) {
                            this.data.entries.splice(this.query.limit);
                        }
                    } else {
                        // Update timestamp even if no new entries
                        lastTimestamp = now;
                    }
                });
            };
            
            // Initial poll and set up interval for every 2 seconds
            pollLogs();
            this.liveInterval = setInterval(pollLogs, 2000);
        },
        stopLive() {
            if (this.liveInterval) {
                clearInterval(this.liveInterval);
                this.liveInterval = null;
            }
            this.live = false;
            console.log('Stopped live logs polling');
        },
        toggleLive() {
            if (this.live) {
                this.stopLive();
            } else {
                if (this.query.view === 'messages' && this.configured) {
                    this.data.entries = [];
                    this.startLive();
                }
            }
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
</style>
