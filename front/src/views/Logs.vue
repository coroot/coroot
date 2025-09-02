<template>
    <Views :error="error" class="logs">
        <v-alert v-if="view.message" color="info" outlined text class="message">
            {{ view.message }}
        </v-alert>

        <template v-else>
            <v-alert v-if="view.error" color="error" icon="mdi-alert-octagon-outline" outlined text class="mt-2">
                {{ view.error }}
            </v-alert>

            <v-card outlined class="px-4 py-2 mb-2">
                <div class="subtitle-1">Query:</div>
                <div class="d-flex flex-wrap flex-md-nowrap gap-2">
                    <QueryBuilder
                        v-model="query.filters"
                        :loading="qb.loading"
                        :items="qb.items"
                        :error="qb.error"
                        :disabled="query.view !== 'messages'"
                        @get="qbGet"
                        class="flex-grow-1"
                    />
                    <v-btn @click="get" :disabled="disabled" color="primary" height="40">Show logs</v-btn>
                    <v-btn @click="toggleLive" :color="live ? 'green' : ''" outlined height="40">
                        <v-icon small class="mr-1">{{ live ? 'mdi-pause' : 'mdi-play' }}</v-icon>
                        {{ live ? 'Live: ON' : 'Live: OFF' }}
                    </v-btn>
                </div>
                <div class="d-flex gap-2 sources">
                    <v-checkbox v-model="query.agent" label="Container logs" :disabled="disabled" dense hide-details />
                    <v-checkbox v-model="query.otel" label="OpenTelemetry" :disabled="disabled" dense hide-details />
                </div>
                <v-progress-linear v-if="loading" indeterminate height="4" style="position: absolute; bottom: 0; left: 0" />
            </v-card>

            <v-tabs height="32" show-arrows hide-slider class="mt-4">
                <v-tab v-for="v in views" :key="v.name" @click="openView(v.name)" class="view" :class="{ active: query.view === v.name }">
                    <v-icon small class="mr-1">{{ v.icon }}</v-icon>
                    {{ v.title }}
                </v-tab>
            </v-tabs>

            <Chart v-if="view.chart" :chart="view.chart" :selection="{}" @select="zoom" class="my-3" />

            <div v-if="query.view === 'messages'">
                <v-simple-table v-if="entries" dense class="entries">
                    <thead>
                        <tr>
                            <th class="px-2">Date</th>
                            <th class="px-2">Application</th>
                            <th class="px-2">Message</th>
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
                            <td class="text-no-wrap px-2">
                                <v-menu offset-y>
                                    <template #activator="{ on }">
                                        <a v-on="on" class="nowrap" style="display: inline-block; max-width: 20ch">{{ e.application }}</a>
                                    </template>
                                    <v-list dense>
                                        <template v-if="e.attributes['service.name']">
                                            <v-list-item @click="qbAdd('service.name', '=', e.attributes['service.name'])">
                                                <v-icon small class="mr-1">mdi-plus</v-icon>
                                                add to search
                                            </v-list-item>
                                            <v-list-item @click="qbAdd('service.name', '!=', e.attributes['service.name'])">
                                                <v-icon small class="mr-1">mdi-minus</v-icon>
                                                exclude from search
                                            </v-list-item>
                                        </template>
                                        <v-list-item v-if="e.link" :to="e.link">
                                            <v-icon small class="mr-1">mdi-open-in-new</v-icon>
                                            go to application
                                        </v-list-item>
                                    </v-list>
                                </v-menu>
                            </td>
                            <td class="text-no-wrap px-2">{{ e.multiline ? e.message.substr(0, e.multiline) : e.message }}</td>
                        </tr>
                    </tbody>
                </v-simple-table>
                <div v-else-if="!loading" class="pa-3 text-center grey--text">No messages found</div>
                <div v-if="entries && entries.length === query.limit" class="text-right caption grey--text mt-1">
                    The output is capped at
                    <InlineSelect v-model="query.limit" :items="limits" />
                    messages.
                </div>
                <LogEntry v-if="entry" v-model="entry" @filter="qbAdd" />
            </div>
        </template>
    </Views>
</template>

<script>
import Views from '@/views/Views.vue';
import { palette } from '@/utils/colors';
import QueryBuilder from '@/components/QueryBuilder.vue';
import Chart from '@/components/Chart.vue';
import LogEntry from '@/components/LogEntry.vue';
import InlineSelect from '@/components/InlineSelect.vue';

export default {
    components: { Views, InlineSelect, LogEntry, Chart, QueryBuilder },

    data() {
        let q = {};
        try {
            q = JSON.parse(this.$route.query.query || '{}');
        } catch {
            //
        }
        return {
            loading: false,
            error: '',
            view: {},
            live: false,
            liveInterval: null,
            query: {
                view: q.view || 'messages',
                agent: q.agent !== undefined ? q.agent : true,
                otel: q.otel !== undefined ? q.otel : true,
                filters: q.filters || [],
                limit: q.limit || 100,
            },
            limits: [10, 20, 50, 100, 1000],
            entry: null,
            qb: {
                loading: false,
                error: '',
                items: [],
            },
        };
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

    computed: {
        views() {
            return [{ name: 'messages', title: 'messages', icon: 'mdi-format-list-bulleted' }];
        },
        entries() {
            if (!this.view.entries) {
                return null;
            }
            const sorted = [...this.view.entries].sort((a, b) => b.timestamp - a.timestamp);
            return sorted.map((e) => {
                const message = e.message.trim();
                const newline = message.indexOf('\n');
                let application = e.application;
                let link;
                if (e.application.includes(':')) {
                    const id = this.$utils.appId(e.application);
                    application = id.name;
                    link = {
                        name: 'overview',
                        params: { view: 'applications', id: e.application, report: 'Logs' },
                        query: this.$utils.contextQuery(),
                    };
                }
                return {
                    ...e,
                    application,
                    link,
                    message,
                    color: palette.get(e.color),
                    date: this.$format.date(e.timestamp, '{MMM} {DD} {HH}:{mm}:{ss}'),
                    multiline: newline > 0 ? newline : 0,
                };
            });
        },
        disabled() {
            return this.loading || this.query.view !== 'messages';
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
        openView(v) {
            this.query.view = v;
        },
        qbAdd(name, op, value) {
            this.query.view = 'messages';
            this.entry = null;
            this.pattern = null;
            this.query.filters.push({ name, op, value });
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
            this.$api.getOverview('logs', query, (data, error) => {
                this.qb.loading = false;
                if (error || data.status === 'warning') {
                    this.qb.error = error || data.message;
                    return;
                }
                this.qb.items = data.logs.suggest || [];
            });
        },
        get() {
            this.loading = true;
            this.error = '';
            this.view.entries = null;
            if (this.live) {
                this.stopLive();
            }
            this.$api.getOverview('logs', JSON.stringify(this.query), (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.view = data.logs || {};
                if (this.query.view === 'messages' && this.live) {
                    this.startLive();
                }
            });
        },
        buildLogsQuery() {
            return JSON.stringify({
                agent: this.query.agent,
                otel: this.query.otel,
                filters: this.query.filters,
                limit: this.query.limit,
                view: 'messages',
            });
        },
        startLive() {
            if (this.liveInterval) return;

            this.live = true;

            let lastTimestamp = Date.now() * 1000; // Convert to microseconds for ClickHouse

            const pollLogs = () => {
                if (!this.live) return;

                const now = Date.now() * 1000; // Current time in microseconds

                // Adjust time range: from last timestamp + 1μs to now
                const timeParams = {
                    from: Math.floor(lastTimestamp / 1000) + 1, // Convert back to milliseconds and add 1ms
                    to: Math.floor(now / 1000),
                };

                // Create query with time range - same approach as AppLogs.vue
                const liveQuery = {
                    ...this.query,
                    from: timeParams.from,
                    to: timeParams.to,
                };

                // Use $api.getOverview like the regular get() method, but with time range
                this.$api.getOverview('logs', JSON.stringify(liveQuery), (data, error) => {
                    if (error) {
                        console.error('Live logs polling error:', error);
                        return;
                    }

                    if (data.status === 'warning') {
                        console.warn('Live logs warning:', data.message);
                        return;
                    }

                    // Process new entries - check both possible structures
                    const entries = data.logs?.entries || data.entries;
                    if (entries && entries.length > 0) {
                        const newEntries = entries.map((e) => {
                            const message = (e.message || '').trim();
                            const newline = message.indexOf('\n');
                            let application = e.application;
                            let link;

                            if (application && application.includes(':')) {
                                const id = this.$utils.appId(application);
                                application = id.name;
                                link = {
                                    name: 'overview',
                                    params: { view: 'applications', id: e.application, report: 'Logs' },
                                    query: this.$utils.contextQuery(),
                                };
                            }

                            return {
                                ...e,
                                application,
                                link,
                                message,
                                color: palette.get(e.color),
                                date: this.$format.date(e.timestamp, '{MMM} {DD} {HH}:{mm}:{ss}'),
                                multiline: newline > 0 ? newline : 0,
                            };
                        });

                        if (!this.view.entries) {
                            this.$set(this.view, 'entries', []);
                        }

                        // Add new entries to the end (chronological order)
                        this.view.entries.push(...newEntries);

                        // Update last timestamp to the newest entry
                        const timestamps = entries.map((e) => new Date(e.timestamp).getTime() * 1000);
                        if (timestamps.length > 0) {
                            lastTimestamp = Math.max(...timestamps);
                        }

                        // Keep limit of entries on screen - remove oldest entries
                        if (this.view.entries.length > this.query.limit) {
                            this.view.entries.splice(0, this.view.entries.length - this.query.limit);
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
        },
        toggleLive() {
            if (this.live) {
                this.stopLive();
            } else {
                if (this.query.view === 'messages') {
                    this.view.entries = [];
                }
                this.startLive();
            }
        },
        zoom(s) {
            const { from, to } = s.selection;
            const query = { ...this.$route.query, from, to };
            this.$router.push({ query }).catch((err) => err);
        },
    },
};
</script>

<style scoped>
.view {
    color: var(--text-color-dimmed);
}
.view.active {
    color: var(--text-color);
    border-bottom: 2px solid var(--text-color);
}
.sources:deep(.v-input--selection-controls__input) {
    margin-right: 0 !important;
}
.mono {
    font-family: monospace, monospace;
}
.marker {
    height: 20px;
    width: 4px;
    filter: brightness(var(--brightness));
}
*:deep(.v-list-item) {
    min-height: 32px !important;
    padding: 0 8px !important;
}
</style>
