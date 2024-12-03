<template>
    <div>
        <div
            v-if="hyp.children.length || hyp.timeseries"
            class="item"
            @click="details = true"
            :style="{ cursor: (hyp.widgets || hyp.log_pattern) && 'pointer' }"
        >
            <div class="nameColumn" :style="nameColumn">
                <!-- eslint-disable-next-line vue/no-unused-vars -->
                <span v-for="_ in hyp.level" class="indent"></span>
                <span>
                    <template v-if="hyp.children.length">
                        <v-icon v-if="folded" @click.stop="folded = false" small class="icon">mdi-chevron-right</v-icon>
                        <v-icon v-else small @click.stop="folded = true" class="icon">mdi-chevron-down</v-icon>
                    </template>
                    <v-icon v-else small>mdi-circle-small</v-icon>
                    <span class="service" :style="service">
                        <template v-if="hyp.name">
                            <span v-html="hyp.name"></span>
                            <span v-if="hyp.disable_reason" class="grey--text caption ml-2">{{ hyp.disable_reason }}</span>
                        </template>
                        <span v-else>{{ $utils.appId(hyp.service).name }}</span>
                    </span>
                </span>
            </div>
            <div class="iconColumn">
                <v-icon v-if="hyp.possible_cause" color="error" small class="mr-1">mdi-alert-circle</v-icon>
            </div>
            <div class="barColumn" :style="barColumn">
                <template v-if="hyp.timeseries">
                    <v-sparkline
                        :value="hyp.timeseries.map((v) => (v === null ? 0 : v))"
                        smooth
                        height="20"
                        fill
                        :color="hyp.disable_reason ? '#d3d3d3' : 'green'"
                        padding="4"
                    />
                </template>
            </div>
        </div>
        <template v-if="!folded">
            <RcaItem v-for="h in hyp.children" :key="h.id" :hyp="h" :split="split" class="child" />
        </template>

        <v-dialog v-if="hyp.widgets || hyp.log_pattern" v-model="details" width="80%">
            <v-card v-if="hyp.log_pattern" class="pa-5">
                <div class="d-flex align-center">
                    <div class="d-flex">
                        <v-chip label dark small :color="palette().get(severity(hyp.log_pattern.severity).color)" class="text-uppercase mr-2">{{
                            hyp.log_pattern.severity
                        }}</v-chip>
                        {{ hyp.log_pattern.sum }} events
                    </div>
                    <v-spacer />
                    <v-btn icon @click="details = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <Chart v-if="hyp.log_pattern.chart" :chart="hyp.log_pattern.chart" />
                <div class="font-weight-medium my-3">Sample</div>
                <div class="message" :class="{ multiline: hyp.log_pattern.multiline }">
                    {{ hyp.log_pattern.sample }}
                </div>
                <v-btn color="primary" :to="showLogMessages(hyp)" class="mt-4"> Show messages </v-btn>
            </v-card>

            <v-card v-else class="pa-5">
                <div class="text-h6 d-flex pb-5">
                    <span>Details</span>
                    <v-spacer />
                    <v-btn icon @click="details = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>
                <div class="d-flex flex-wrap">
                    <div
                        v-for="w in hyp.widgets"
                        class="my-5"
                        :style="{ width: $vuetify.breakpoint.mdAndUp && hyp.widgets.length > 1 ? w.width || '50%' : '100%' }"
                    >
                        <Widget :w="w" />
                    </div>
                </div>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
import Widget from '@/components/Widget.vue';
import Chart from '@/components/Chart.vue';
import { palette } from '@/utils/colors';

export default {
    name: 'RcaItem',
    methods: {
        palette() {
            return palette;
        },
        severity(s) {
            s = s.toLowerCase();
            if (s.startsWith('crit')) return { num: 5, color: 'black' };
            if (s.startsWith('err')) return { num: 4, color: 'red-darken1' };
            if (s.startsWith('warn')) return { num: 3, color: 'orange-lighten1' };
            if (s.startsWith('info')) return { num: 2, color: 'blue-lighten2' };
            if (s.startsWith('debug')) return { num: 1, color: 'green-lighten1' };
            return { num: 0, color: 'grey-lighten1' };
        },
        showLogMessages(hyp) {
            return {
                name: 'overview',
                params: {
                    view: 'applications',
                    id: hyp.service,
                    report: 'Logs',
                },
                query: {
                    query: JSON.stringify({ hash: hyp.log_pattern.hash, view: 'messages' }),
                    from: this.$route.query.rcaFrom,
                    to: this.$route.query.rcaTo,
                },
            };
        },
    },

    components: { Widget, Chart },

    props: {
        hyp: Object,
        split: Number,
    },

    data() {
        return {
            folded: false,
            details: false,
        };
    },

    computed: {
        nameColumn() {
            return { width: this.split + '%' };
        },
        barColumn() {
            return { width: 100 - this.split + '%' };
        },
        service() {
            return { borderColor: this.hyp.color };
        },
    },
};
</script>

<style scoped>
.item {
    display: flex;
    line-height: 2;
}
.item:hover,
.item:hover ~ .child {
    background-color: var(--background-color-hi);
}
.nameColumn {
    display: flex;
    white-space: nowrap;
    overflow: hidden;
    padding-right: 8px;
}
.indent {
    padding-right: 16px;
    display: inline-flex;
    height: 100%;
}
.indent::before {
    content: '';
    margin-left: 7px;
    padding-left: 1px;
    background-color: lightgrey;
}
.service {
    position: relative;
    padding-left: 8px;
}
.service::before {
    content: '';
    position: absolute;
    top: 3px;
    bottom: 3px;
    left: 0;
    border-left-width: 4px;
    border-left-style: solid;
    border-left-color: inherit;
}
.iconColumn {
    width: 24px;
    padding-left: 8px;
}
.nameColumn:deep(.logline) {
    font-size: 14px;
    background-color: var(--background-color-hi);
    padding: 4px 8px;
    //border-radius: 3px;
}
.barColumn {
    display: flex;
    position: relative;
}
.icon:focus::after {
    opacity: 0;
}
.message {
    font-family: monospace, monospace;
    font-size: 14px;
    background-color: var(--background-color-hi);
    border-radius: 3px;
    max-height: 50vh;
    padding: 8px;
    overflow: auto;
}
.message.multiline {
    white-space: pre;
}
</style>
