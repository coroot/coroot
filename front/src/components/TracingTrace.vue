<template>
    <div v-if="roots.length">
        <div class="details mb-3">
            <span class="grey--text">Started at:</span> {{$format.date(range.from, '{YYYY}-{MM}-{DD} {HH}:{mm}:{ss}.{fff}')}}
            <span class="grey--text ml-1">Duration:</span> {{(range.to - range.from).toFixed(2)}}ms
            <span class="grey--text ml-1">Status:</span>
            <v-icon v-if="roots[0].status.error" color="error" small class="ml-1" style="margin-bottom: 2px">mdi-alert-circle</v-icon>
            <v-icon v-else color="success" small class="ml-1" style="margin-bottom: 2px">mdi-check-circle</v-icon>
            <span>{{roots[0].status.message}}</span>
        </div>
        <div class="header">
            <div class="name" :style="{width: split+'%'}">
                Service & Operation
                <v-spacer />
                <v-btn @click="full = !full" x-small plain color="primary" class="caption">
                    {{full ? 'show sub-trace' : 'show full trace'}}
                </v-btn>
            </div>
            <div class="ticks" :style="{width: 100-split+'%'}">
                <div v-for="t in ticks" class="tick grey--text caption" :style="{width: t.width+'%'}">
                    {{t.value}}
                </div>
            </div>
        </div>
        <TracingSpan v-for="s in roots" :key="s.id" :span="s" :ticks="ticks" :split="split" />
    </div>
</template>

<script>
import {palette} from "@/utils/colors";
import TracingSpan from "@/components/TracingSpan";

const nameColumnWidth = 50; // %
const barsAreaWidth = 80; // %
const ticksCount = 5;

export default {
    props: {
        spans: Array,
        span: String,
    },

    components: {TracingSpan},

    data() {
        return {
            full: false,
        };
    },

    computed: {
        split() {
            return nameColumnWidth;
        },
        tree() {
            if (!this.spans.length) {
                return [];
            }
            const byId = new Map();
            this.spans.forEach(s => {
                byId.set(s.id, s);
            })
            const f = (s, parent) => {
                const span = {
                    id: s.id,
                    name: s.name,
                    status: s.status,
                    children: [],
                    level: parent.level+1,
                    service: s.service,
                    color: palette.hash2(s.service),
                    timestamp: s.timestamp,
                    duration: s.duration,
                    attributes: s.attributes,
                };
                parent.children.push(span);
                this.spans.filter(s => s.parent_id === span.id).forEach(s => {
                    f(s, span);
                })
            }
            const tree = {level: -1, children: []};
            if (!this.full && this.span) {
                this.spans.filter(s => s.id === this.span).forEach(s => {
                    f(s, tree);
                })
            } else {
                this.spans.filter(s => !s.parent_id || !byId.has(s.parent_id)).forEach(s => {
                    f(s, tree);
                })
            }
            return tree.children;
        },
        range() {
            if (!this.tree.length) {
                return null;
            }
            const range = {from: Infinity, to: 0};
            const f = (s) => {
                range.from = Math.min(range.from, s.timestamp);
                range.to = Math.max(range.to, s.timestamp+s.duration);
                s.children.forEach(f);
            }
            this.tree.forEach(f);
            return range;
        },
        roots() {
            if (!this.spans.length) {
                return [];
            }
            const duration = this.range.to - this.range.from;
            const f = (s) => {
                s.offset = (s.timestamp-this.range.from)*barsAreaWidth/duration;
                s.width = s.duration*barsAreaWidth/duration;
                s.children.forEach(f);
            }
            this.tree.forEach(f);
            return this.tree;
        },
        ticks() {
            if (!this.range) {
                return [];
            }
            const v = (this.range.to - this.range.from) / ticksCount;
            const w = barsAreaWidth / ticksCount;
            const fmt = (v) => {
                if (!v) {
                    return '0';
                }
                if (v > 1000) {
                    return (v/1000).toFixed(2)+'s';
                }
                return v.toFixed(2)+'ms';
            }
            return Array.from({length: ticksCount+1}).map((_, i) => {
                return {
                    value: fmt(v * i),
                    width: w,
                };
            });
        },
    },
}
</script>

<style scoped>
.header {
    display: flex;
}
.name {
    display: flex;
}
.ticks {
    display: flex;
    border-bottom: 1px solid rgba(0,0,0,0.1);
}
.tick {
    height: 100%;
    border-left: 1px solid rgba(0,0,0,0.1);
    padding-left: 2px;
}
</style>