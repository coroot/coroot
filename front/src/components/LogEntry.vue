<template>
    <v-dialog v-model="dialog" width="80%">
        <v-card class="pa-5">
            <div class="d-flex align-center">
                <div class="d-flex">
                    <v-chip label dark small :color="value.color" class="text-uppercase mr-2">{{ value.severity }}</v-chip>
                    {{ value.date }}
                </div>
                <v-spacer />
                <v-btn icon @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
            </div>

            <div class="font-weight-medium my-3">Message</div>
            <div class="message" :class="{ multiline: value.multiline }">
                {{ value.message }}
            </div>

            <div class="font-weight-medium mt-4 mb-2">Attributes</div>
            <v-simple-table dense class="attributes">
                <tbody>
                    <tr v-for="(v, k) in value.attributes">
                        <td class="name">{{ k }}</td>
                        <td class="value">
                            <router-link
                                v-if="k === 'host.name'"
                                :to="{ name: 'overview', params: { view: 'nodes', id: v }, query: $utils.contextQuery() }"
                            >
                                {{ v }}
                            </router-link>
                            <div v-else class="value">{{ v }}</div>
                        </td>
                        <td class="text-right text-no-wrap ops">
                            <v-btn small icon title="add to search" @click="filter(k, '=', v)">
                                <v-icon small>mdi-plus</v-icon>
                            </v-btn>
                            <v-btn small icon title="exclude from search" @click="filter(k, '!=', v)">
                                <v-icon small>mdi-minus</v-icon>
                            </v-btn>
                        </td>
                    </tr>
                </tbody>
            </v-simple-table>
            <v-btn
                v-if="value.attributes['pattern.hash']"
                color="primary"
                @click="filter('pattern.hash', '=', value.attributes['pattern.hash'])"
                class="mt-4"
            >
                Show similar messages
            </v-btn>
            <v-btn v-if="traceLink" color="primary" :to="traceLink" class="mt-4"> Show the trace </v-btn>
        </v-card>
    </v-dialog>
</template>

<script>
export default {
    props: {
        value: Object,
        appId: String,
    },

    data() {
        return {
            dialog: !!this.value,
        };
    },

    watch: {
        dialog(v) {
            !v && this.$emit('input', null);
        },
    },

    computed: {
        traceLink() {
            if (!this.value.trace_id) {
                return null;
            }
            if (this.appId) {
                return {
                    params: { view: 'applications', id: this.appId, report: 'Tracing' },
                    query: { query: undefined, trace: 'otel:' + this.value.trace_id + ':-:-:' },
                };
            }
            return {
                params: { view: 'traces' },
                query: { query: JSON.stringify({ view: 'traces', trace_id: this.value.trace_id }) },
            };
        },
    },

    methods: {
        filter(name, op, value) {
            this.$emit('filter', name, op, value);
        },
    },
};
</script>

<style scoped>
.attributes td {
    padding: 0 4px !important;
}
.attributes .value {
    word-break: break-word;
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
