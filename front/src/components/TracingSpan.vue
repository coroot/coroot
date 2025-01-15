<template>
    <div>
        <div class="span" @click="details = true">
            <div class="nameColumn" :style="nameColumn">
                <!-- eslint-disable-next-line vue/no-unused-vars -->
                <span v-for="_ in span.level" class="indent"></span>
                <span>
                    <template v-if="span.children.length">
                        <v-icon v-if="folded" @click.stop="folded = false" small class="icon">mdi-chevron-right</v-icon>
                        <v-icon v-else small @click.stop="folded = true" class="icon">mdi-chevron-down</v-icon>
                    </template>
                    <v-icon v-else small>mdi-circle-small</v-icon>
                    <span class="service" :style="service">{{ span.service }}</span>
                    <span class="caption grey--text ml-1">{{ span.name }}</span>
                </span>
                <v-spacer />
                <v-icon v-if="span.status.error" color="error" small class="mr-1">mdi-alert-circle</v-icon>
            </div>
            <div class="barColumn" :style="barColumn">
                <div v-for="(t, i) in ticks" class="tick" :style="{ left: i * t.width + '%' }"></div>
                <div class="bar" :style="bar"></div>
                <div class="caption grey--text my-auto ml-1">{{ span.duration.toFixed(2) }}ms</div>
            </div>
        </div>
        <template v-if="!folded">
            <TracingSpan v-for="s in span.children" :key="s.id" :span="s" :ticks="ticks" :split="split" class="child" />
        </template>

        <v-dialog v-model="details" width="80%">
            <v-card class="pa-5">
                <div class="text-h6 d-flex">
                    <span>Span: {{ span.id }}</span>
                    <v-spacer />
                    <v-btn icon @click="details = false"><v-icon>mdi-close</v-icon></v-btn>
                </div>

                <v-simple-table dense>
                    <tbody>
                        <tr>
                            <td>name</td>
                            <td>
                                <pre>{{ span.name }}</pre>
                            </td>
                        </tr>
                        <tr>
                            <td>service</td>
                            <td>
                                <pre>{{ span.service }}</pre>
                            </td>
                        </tr>
                        <tr>
                            <td>duration</td>
                            <td>
                                <pre>{{ span.duration.toFixed(2) }}ms</pre>
                            </td>
                        </tr>
                        <tr>
                            <td>status</td>
                            <td>
                                <div class="d-flex" style="gap: 4px">
                                    <v-icon v-if="span.status.error" color="error" small>mdi-alert-circle</v-icon>
                                    <v-icon v-else color="success" small>mdi-check-circle</v-icon>
                                    <pre>{{ span.status.message }}</pre>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </v-simple-table>

                <div class="font-weight-medium mt-4">Attributes</div>
                <v-simple-table dense>
                    <tbody>
                        <tr v-for="(v, k) in span.attributes">
                            <td>{{ k }}</td>
                            <td>
                                <pre>{{ v }}</pre>
                            </td>
                        </tr>
                    </tbody>
                </v-simple-table>

                <div v-for="e in span.events" class="event mt-4">
                    <div>
                        <span class="font-weight-medium">Event:</span> {{ e.name }}
                        <span class="caption grey--text">{{ e.timestamp - span.timestamp }}ms since span start</span>
                    </div>
                    <v-simple-table dense>
                        <tbody>
                            <tr v-for="(v, k) in e.attributes">
                                <td>{{ k }}</td>
                                <td>
                                    <pre>{{ v }}</pre>
                                </td>
                            </tr>
                        </tbody>
                    </v-simple-table>
                </div>
            </v-card>
        </v-dialog>
    </div>
</template>

<script>
export default {
    name: 'TracingSpan',

    props: {
        span: Object,
        ticks: Array,
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
            return { borderColor: this.span.color };
        },
        bar() {
            return {
                marginLeft: this.span.offset + '%',
                width: this.span.width + '%',
                backgroundColor: this.span.color,
            };
        },
    },
};
</script>

<style scoped>
.span {
    display: flex;
    cursor: pointer;
    line-height: 2;
}
.span:hover,
.span:hover ~ .child {
    background-color: var(--background-color-hi);
}
.nameColumn {
    display: flex;
    white-space: nowrap;
    overflow: hidden;
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
.barColumn {
    display: flex;
    position: relative;
}
.tick {
    position: absolute;
    width: 1px;
    height: 100%;
    background-color: var(--border-color);
}
.bar {
    height: 12px;
    margin: auto 0;
    z-index: 0;
}
.icon:focus::after {
    opacity: 0;
}
</style>
