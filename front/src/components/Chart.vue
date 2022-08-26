<template>
    <div ref="container" class="chart">
        <div class="title">
            <slot name="title">
                <span v-html="title"></span>
            </slot>
        </div>

        <div ref="uplot" v-on-resize="redraw" style="position: relative">
            <div v-for="f in flags" class="flag" :style="{left: f.left+'px', height: f.height+'px'}">
                <v-tooltip bottom>
                    <template #activator="{on}">
                        <v-icon v-on="on" small style="z-index: 2">{{f.icon}}</v-icon>
                    </template>
                    <div v-html="f.type" class="text-center"/>
                </v-tooltip>
                <div class="line" style="z-index: 2"></div>
            </div>
        </div>

        <div ref="tooltip" style="display: none; position: absolute; z-index: 1;">
            <div v-if="tooltip" class="tooltip">
                <div class="ts">{{tooltip.ts}}</div>
                <div v-for="i in tooltip.items" class="item">
                    <span class="marker" :style="{backgroundColor: i.color}" />
                    <span class="label">{{i.label}}:</span>
                    <span class="value">{{i.value}}</span>
                </div>
                <div v-if="tooltip.outage" class="outage">
                    <span class="label">outage</span>:
                    {{tooltip.outage.from}} - {{tooltip.outage.to || 'in progress'}} ({{tooltip.outage.dur}})
                </div>
            </div>
        </div>

        <div v-if="legend" class="legend">
            <div v-for="l in legend" class="item" :style="{opacity: l.hidden ? '0.5' : '1'}"
                  @click="toggleSeries(l.label)" @mouseover="highlightSeries(l.label)" @mouseleave="highlightSeries(null)">
                <span class="marker" :style="{backgroundColor: l.color}"></span>
                <span class="label">{{l.label}}</span>
            </div>
        </div>
    </div>
</template>

<script>
import 'uplot/dist/uPlot.min.css';
import uPlot from 'uplot';
import {palette} from "@/utils/colors";
import convert from 'color-convert';

const font = '12px Poppins, sans-serif'
const tsFormat = '{MMM} {DD}, {HH}:{mm}:{ss}';
const fmtDate = uPlot.fmtDate(tsFormat);
const fmtTs = (ts) => fmtDate(new Date(ts));

const suffixes1 = ['', 'K', 'M', 'G', 'T', 'P', 'E', 'Z', 'Y'];
const suffixes2 = ['', 'm', 'µ', 'n', 'p', 'f', 'a', 'z', 'y'];
const fmtVal = (max, unit, digits) => {
    const p = !max ? 0 : Math.floor(Math.log(max) / Math.log(1000));
    const suffix = p > 0 ? suffixes1[p] : suffixes2[-p];

    return (v) => {
        if (v === null || isNaN(v)) {
            return '-';
        }
        let res = p < 0 ? v * Math.pow(1000, -p) : v / Math.pow(1000, p);
        if (digits !== undefined) {
           res = res.toFixed(digits);
        }
        return res + suffix + (unit || '');
    };
};

export default {
    props: {
        chart: Object,
    },

    data() {
        return {
            ch: null,
            idx: null,
            hiddenSeries: (this.chart.series || []).filter((s) => s.hidden).map((s) => s.name),
            highlightedSeries: null,
        };
    },
    mounted() {
        this.$nextTick(this.redraw);
    },
    beforeDestroy() {
        this.ch && this.ch.destroy();
    },

    watch: {
        config() {
            this.$nextTick(this.redraw);
        },
    },

    computed: {
        config() {
            const c = JSON.parse(JSON.stringify(this.chart));
            c.series = (c.series || []).filter((s) => s.data != null);

            if (!c.sorted) {
                c.series.sort((a, b) => a.name.localeCompare(b.name));
            }
            delete c.sorted;

            if (c.threshold) {
                c.threshold.stacked = false;
                c.series.push(c.threshold);
                delete c.threshold;
            }

            c.ctx.data = Array.from({length: (c.ctx.to - c.ctx.from) / c.ctx.step + 1}, (_, i) => c.ctx.from + (i * c.ctx.step));

            const colors = {};
            c.series.filter((s) => s.color).forEach((s, i) => {
                if (!colors[s.color]) {
                    colors[s.color] = [];
                }
                colors[s.color].push(i);
            });

            c.series.forEach((s, i) => {
                s.stacked = s.stacked !== undefined ? s.stacked : c.stacked;
                s.data = s.data.values;
                if (colors[s.color] && colors[s.color].length > 1) {
                    const c = palette.get(s.color, 0);
                    const hsl = convert.hex.hsl(c);
                    const idx = colors[s.color].findIndex((ii) => ii === i);
                    hsl[2] = 70 - Math.trunc(idx * 30 / colors[s.color].length)
                    // console.log(s.color, c, convert.hex.hsl(c), hsl)
                    s.color = '#' + convert.hsl.hex(hsl);
                } else {
                    s.color = palette.get(s.color, i + (c.color_shift || 0));
                }
                s.fill = s.stacked || s.fill;
            });
            delete c.stacked;

            if (c.annotations) {
                c.outages = c.annotations.filter((a) => a.name === 'outage').map((a) => ({x1: a.x1, x2: a.x2}));
                c.flags = c.annotations.filter((a) => a.name !== 'outage').map((a) => ({msg: a.name, x: a.x1, icon: a.icon}));
            }
            delete c.annotations;
            return c;
        },
        flags() {
            if (!this.config || !this.config.flags || !this.ch) {
                return [];
            }
            const c = this.config;
            const b = this.ch.bbox;
            const norm = (x) => (x - c.ctx.from) / (c.ctx.to - c.ctx.from);
            return c.flags.map((f) => {
                const type = f.msg;
                const left = (b.left + b.width * norm(f.x)) / window.devicePixelRatio;
                const height = (b.top + b.height) / window.devicePixelRatio;
                const icon = f.icon || 'mdi-alert-circle-outline';
                return {type, left, height, icon};
            });
        },
        title() {
            return this.config.title;
        },
        tooltip() {
            if (this.idx === null) {
                return null;
            }
            const c = this.config;
            const ss = c.series.filter(this.isActive);
            const max = ss.reduce((p, c) => Math.max(p, ...c.data), null);
            const f = fmtVal(max, c.unit, 2);
            const ts = c.ctx.data[this.idx];
            const o = (c.outages || []).find((o) => o.x1 <= ts && ts <= o.x2);
            return {
                ts: fmtTs(ts),
                items: ss.map((s) => ({label: s.name, value: f(s.data[this.idx]), color: s.color})),
                outage: o && {
                    from: fmtTs(o.x1),
                    to: o.x2 < c.ctx.to && fmtTs(o.x2),
                    dur: this.$moment.duration((o.x2-o.x1 + c.ctx.step), 'ms').format('h[h] m[m] s[s]', {trim: 'all'}),
                },
            }
        },
        legend() {
            const c = this.config;
            if (c.hide_legend) {
                return null;
            }
            return c.series.map((s) => ({label: s.name, color: s.color, hidden: !this.isActive(s)}));
        },
    },

    methods: {
        redraw() {
            const c = this.config;
            const ss = c.series.filter(this.isActive);
            const f = (s) => ({
                label: s.name,
                stroke: !s.stacked && s.color,
                width: c.column ? 0 : 2,
                fill: s.fill && s.color + (s.stacked ? 'ff' : '44'),
                points: {show: false},
                paths: c.column && uPlot.paths.bars(),
            });
            const series = [];
            const data = [];
            const a = Array(c.ctx.data.length).fill(0);
            ss.forEach((_, i) => {
                const s = ss[ss.length - 1 - i];
                if (!s.stacked) {
                    return;
                }
                series.unshift(f(s));
                data.unshift(s.data.map((v, i) => a[i] += v));
            });
            ss.filter((s) => !s.stacked).forEach((s) => {
                if (s.fill) {
                    series.unshift(f(s));
                    data.unshift(s.data);
                } else {
                    series.push(f(s));
                    data.push(s.data);
                }
            });
            if (!(c.yzoom)) { // fake series to show y == 0
                series.push({});
                data.push([0]);
            }
            const opts = {
                height: 150,
                width: this.$refs.uplot.clientWidth,
                ms: 1,
                axes: [
                    // {space: 80, font, values: tsFormat},
                    {space: 80, font, values: [
                            [60 * 1000, "{HH}:{mm}", null, null, "{MMM} {DD}", null, null, null, 0],
                            [1 * 1000, "{HH}:{mm}:{ss}", null, null, "{MMM} {DD}", null, null, null, 0],
                        ],
                    },
                    {space: 20, font, size: 60, values: (u, splits) => splits.map(v => fmtVal(Math.max(...splits), c.unit)(v))},
                ],
                series: [{}, ...series],
                cursor: {points: {show: false}, y: false},
                legend: {show: false},
                hooks: {
                    draw: [
                        this.drawOutages,
                        // this.drawFlags,
                    ],
                },
                plugins: [
                    this.tooltipPlugin(),
                ],
            };

            if (this.ch) {
                this.ch.destroy();
            }
            this.ch = new uPlot(opts, [c.ctx.data, ...data], this.$refs.uplot);
            this.ch.root.style.font = font;
        },
        drawOutages(u) {
            const c = this.config;
            if (!c.outages || !c.outages.length) {
                return;
            }
            const norm = (x) => (x - c.ctx.from) / (c.ctx.to - c.ctx.from);
            const b = u.bbox;
            u.ctx.save();
            const h = 3*window.devicePixelRatio;
            const y = b.top + b.height + 3*window.devicePixelRatio;
            u.ctx.fillStyle = 'hsl(141, 50%, 70%)';
            u.ctx.fillRect(b.left, y, b.width, h);
            u.ctx.fillStyle = 'hsl(4, 90%, 60%)';
            c.outages.forEach((o) => {
                const x1 = Math.max(b.left, b.left + b.width * norm(o.x1 - c.ctx.step/2));
                const x2 = Math.min(b.left + b.width, b.left + b.width * norm(o.x2 + c.ctx.step/2));
                u.ctx.fillRect(x1, y,x2-x1, h);
            });
            u.ctx.restore();
        },
        drawFlags(u) {
            const c = this.config;
            if (!c.flags || !c.flags.length) {
                return;
            }
            const norm = (x) => (x - c.ctx.from) / (c.ctx.to - c.ctx.from);
            const b = u.bbox;
            u.ctx.save();
            u.ctx.strokeStyle = 'rgba(0,0,0,0.5)';
            u.ctx.fillStyle = 'rgba(0,0,0,0.7)';
            u.ctx.lineWidth = window.devicePixelRatio;
            u.ctx.setLineDash([4*window.devicePixelRatio]);
            c.flags.forEach((f) => {
                const x = b.left + b.width * norm(f.x);
                u.ctx.save();
                u.ctx.translate(x, b.top-5*window.devicePixelRatio);
                u.ctx.beginPath();
                u.ctx.moveTo(0, b.height+5*window.devicePixelRatio);
                u.ctx.lineTo(0, 0);
                u.ctx.stroke();
                u.ctx.rotate(-Math.PI/2);
                u.ctx.fillText(f.msg, 0, -7*window.devicePixelRatio);
                u.ctx.restore();
            });
            u.ctx.restore();
        },
        tooltipPlugin() {
            const init = (u) => {
                const t = this.$refs.tooltip;
                u.over.appendChild(t);
                u.over.addEventListener("mouseenter", () => t.style.display = 'block');
                u.over.addEventListener("mouseleave", () => t.style.display = 'none');
            }
            const move = (u) => {
                const { left, top, idx } = u.cursor;
                if (idx === null) {
                    return;
                }
                this.idx = idx;
                const t = this.$refs.tooltip;
                const l = left - (left > u.over.clientWidth/2 ? t.clientWidth + 5 : -5);
                t.style.transform = "translate(" + l + "px, " + top + "px)";
            }
            return {hooks: {init: init, setCursor: move}}
        },
        toggleSeries(name) {
            const i = this.hiddenSeries.indexOf(name);
            if (i > -1) {
                this.hiddenSeries.splice(i, 1);
            } else {
                this.hiddenSeries.push(name);
            }
            this.highlightedSeries = null;
            this.redraw();
        },
        highlightSeries(name) {
            this.highlightedSeries = name;
            this.redraw();
        },
        isActive(s) {
            if (this.highlightedSeries) {
                return s.name === this.highlightedSeries;
            }
            return this.hiddenSeries.indexOf(s.name) < 0;
        },
    },
};
</script>

<style scoped>
.title {
    font-size: 14px !important;
    font-weight: normal !important;
    text-align: center;
    line-height: 1.5em;
}
.tooltip {
    background-color: white;
    padding: 8px;
    border: 1px solid rgba(0,0,0,0.2);
    border-radius: 4px;
    pointer-events: none;
    font-size: 12px;
}
.tooltip .ts {
    text-align: center;
}
.tooltip .item {
    display: flex;
    align-items: center;
}
.tooltip .item .marker {
    height: 12px;
    width: 6px;
    margin-right: 4px;
}
.tooltip .item .label {
    max-width: 200px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.tooltip .item .value {
    flex-grow: 2;
    text-align: right;
    margin-left: 8px;
    font-weight: 600;
}
.tooltip .outage {
    margin-top: 10px;
    padding-top: 5px;
    /*color: hsl(4, 90%, 58%);*/
    border-top: 1px solid black;
}
.tooltip .outage .label {
    color: hsl(4, 90%, 58%);
    font-weight: 600;
}

.legend {
    margin: 0 10px 0 40px;
    display: flex;
    flex-wrap: wrap;
    max-height: 36px; /* 2 lines of .items */
    overflow: auto;
}
.legend .item {
    padding-right: 10px;
    font-size: 12px;
    display: flex;
    align-items: center;
    cursor: pointer;
    max-width: 100%;
}
.legend .item .marker {
    width: 6px;
    height: 12px;
    padding-right: 6px;
}
.legend .item .label {
    margin-left: 6px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.flag {
    position: absolute;
    transition: none;
    display: flex;
    flex-direction: column;
    width: 0;
}
.flag .line {
    flex-grow: 1;
    border-left: 0.08rem dashed rgba(0,0,0,0.5);
    margin-left: -0.04rem;
}
</style>
