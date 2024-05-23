<template>
    <div class="heatmap">
        <div class="title" v-html="config.title" />

        <div>
            <div class="legend">
                <div v-for="i in legend" class="item" :class="i.type">
                    {{ i.value }}
                </div>
            </div>
        </div>

        <div ref="uplot" v-on-resize="redraw" class="chart" :class="{ loading: loading }">
            <div class="threshold" style="z-index: 1" :style="threshold.style">
                <v-tooltip left>
                    <template #activator="{ on }">
                        <v-icon v-on="on" small class="icon">mdi-target</v-icon>
                    </template>
                    <v-card v-html="threshold.content" class="pa-2 text-center" />
                </v-tooltip>
            </div>
            <ChartAnnotations :ctx="config.ctx" :bbox="bbox" :annotations="annotations" />
            <ChartIncidents :ctx="config.ctx" :bbox="bbox" :incidents="incidents" />
        </div>

        <ChartTooltip ref="tooltip" v-model="idx" :ctx="config.ctx" :incidents="incidents" class="tooltip">
            <div v-for="i in tooltip" class="item" :class="{ threshold: !!i.threshold }">
                <span v-if="!!i.threshold" class="details">
                    <span class="above">{{ i.threshold.above }}</span>
                    <br />
                    <span class="below">{{ i.threshold.below }}</span>
                </span>
                <div class="label" :style="{ width: i.width + 'ch' }">{{ i.label }}</div>
                <div class="value">
                    <div class="bar" :class="{ error: i.error }" :style="{ width: i.bar + '%' }">{{ i.value }}</div>
                </div>
            </div>
        </ChartTooltip>
    </div>
</template>

<script>
import uPlot from 'uplot';
import ChartTooltip from './ChartTooltip';
import ChartAnnotations from './ChartAnnotations';
import ChartIncidents from './ChartIncidents';

const font = '12px Roboto, sans-serif';

function fmtDigits(...v) {
    const min = Math.min(...v.filter((v) => !!v));
    const p = Math.floor(Math.log(min) / Math.log(10));
    if (p >= 0) {
        return 0;
    }
    return -p;
}

function fmtVal(v, unit, digits) {
    if (!v) {
        return '-';
    }
    if (digits === undefined) {
        digits = fmtDigits(v);
    }
    return v.toFixed(digits) + unit;
}

export default {
    props: {
        heatmap: Object,
        selection: Object,
        loading: Boolean,
    },

    components: { ChartTooltip, ChartAnnotations, ChartIncidents },

    data() {
        return {
            ch: null,
            bbox: null,
            idx: null,
            select: {},
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
        theme() {
            this.redraw();
        },
    },

    computed: {
        theme() {
            const dark = this.$vuetify.theme.dark;
            return {
                text: dark ? 'rgba(255, 255, 255, 0.4)' : 'rgba(0,0,0,0.87)',
                grid: dark ? 'rgba(255, 255, 255, 0.3)' : 'rgba(0,0,0,0.07)',
            };
        },
        config() {
            const c = JSON.parse(JSON.stringify(this.heatmap));
            c.series = (c.series || []).filter((s) => s.data != null);
            c.ctx.data = Array.from({ length: (c.ctx.to - c.ctx.from) / c.ctx.step + 1 }, (_, i) => c.ctx.from + i * c.ctx.step);
            c.ctx.min = Math.min(...c.series.map((s) => Math.min(...s.data.filter((v) => !!v))));
            c.ctx.max = Math.max(...c.series.map((s) => Math.max(...s.data)));
            return c;
        },
        legend() {
            const c = this.config;
            if (!c) {
                return [];
            }
            const { min, max } = c.ctx;
            if (min === Infinity) {
                return [];
            }
            const avg = (min + max) / 2;
            return [
                { type: 'min', value: fmtVal(min, '/s') },
                { type: 'avg', value: fmtVal(avg, '/s') },
                { type: 'max', value: fmtVal(max, '/s') },
            ];
        },
        threshold() {
            const none = { style: { display: 'none' } };
            const c = this.config;
            if (!this.ch || !c) {
                return none;
            }
            const idx = c.series.findIndex((s) => !!s.threshold);
            if (idx === -1) {
                return none;
            }
            const b = this.bbox;
            const h = b.height / c.series.length;
            return {
                content: c.series[idx].threshold,
                style: {
                    display: 'block',
                    top: b.top + b.height - h * (idx + 1) - 1 + 'px',
                    left: b.left + 'px',
                    width: b.width + 5 + 'px',
                },
            };
        },
        annotations() {
            return (this.config.annotations || []).filter((a) => a.name !== 'incident').map((a) => ({ msg: a.name, x: a.x1, icon: a.icon }));
        },
        incidents() {
            return (this.config.annotations || []).filter((a) => a.name === 'incident').map((a) => ({ x1: a.x1, x2: a.x2 }));
        },
        tooltip() {
            const c = this.config;
            if (!c || this.idx === null) {
                return [];
            }
            const idx = this.idx;
            const max = Math.max(...c.series.map((s) => s.data[idx]));
            let threshold = null;
            const thresholdIdx = c.series.findIndex((s) => !!s.threshold);
            if (thresholdIdx > -1) {
                let below = c.series.filter((_, i) => i <= thresholdIdx).reduce((sum, s) => sum + s.data[idx], 0);
                let above = c.series.filter((_, i) => i > thresholdIdx).reduce((sum, s) => sum + s.data[idx], 0);
                const total = below + above;
                below = (below * 100) / total;
                above = (above * 100) / total;
                const digits = fmtDigits(below, above);
                threshold = {
                    below: fmtVal(below, '%', digits),
                    above: fmtVal(above, '%', digits),
                };
            }
            const width = Math.max(...c.series.map((s) => s.title.length));
            return c.series
                .map((s, i) => {
                    return {
                        label: s.title,
                        error: s.value === 'err',
                        value: fmtVal(s.data[idx], '/s'),
                        bar: s.data[idx] ? Math.trunc((s.data[idx] * 100) / max) : 0,
                        threshold: i === thresholdIdx && threshold,
                        width,
                    };
                })
                .reverse();
        },
    },

    methods: {
        redraw() {
            const c = this.config;
            const hm = this.heatmapPaths();
            const opts = {
                height: 250,
                width: this.$refs.uplot.clientWidth,
                padding: [20, 20, 0, 0],
                ms: 1,
                scales: {
                    y: {
                        min: 0,
                        max: c.series.length,
                    },
                },
                axes: [
                    {
                        space: 80,
                        font,
                        stroke: this.theme.text,
                        grid: { stroke: this.theme.grid },
                        ticks: { stroke: this.theme.grid },
                        values: [
                            [60000, '{HH}:{mm}', null, null, '{MMM} {DD}', null, null, null, 0],
                            [1000, '{HH}:{mm}:{ss}', null, null, '{MMM} {DD}', null, null, null, 0],
                        ],
                    },
                    {
                        scale: 'y',
                        font,
                        gap: 0,
                        size: 60,
                        stroke: this.theme.text,
                        grid: { stroke: this.theme.grid },
                        ticks: { stroke: this.theme.grid },
                        splits: [0, ...c.series.map((_, i) => i + 1)],
                        values: ['0', ...c.series.map((s) => s.name)],
                    },
                ],
                series: [{}, ...c.series.map(() => ({ paths: hm, alpha: 0 }))],
                cursor: {
                    points: { show: false },
                    y: !!this.selection,
                    drag: { setScale: false, x: !!this.selection, y: !!this.selection },
                    bind: {
                        dblclick: () => () => null, // avoid some strange collapse of the y-axis
                    },
                    lock: true,
                },
                legend: { show: false },
                plugins: [this.$refs.tooltip.plugin(), this.selectionPlugin()],
            };

            if (this.ch) {
                this.ch.destroy();
            }
            this.ch = new uPlot(opts, [c.ctx.data, ...c.series.map((s) => s.data)], this.$refs.uplot);
            this.ch.root.style.font = font;
            this.bbox = Object.entries(this.ch.bbox).reduce((o, e) => {
                o[e[0]] = e[1] / devicePixelRatio;
                return o;
            }, {});
        },
        heatmapPaths() {
            const c = this.config;
            const norm = c.ctx.max - c.ctx.min;
            const margin = 1;
            return (u, seriesIdx) => {
                const xs = u.data[0];
                const ys = u.data[seriesIdx];
                const h = u.bbox.height / c.series.length;
                const w = u.bbox.width / xs.length;
                const y = u.bbox.height + u.bbox.top - seriesIdx * h;
                const x = u.bbox.left - w / 2 + margin / 2;
                const baselineColor = c.series[seriesIdx - 1].value === 'err' ? '0' : '200';
                uPlot.orient(
                    u,
                    seriesIdx,
                    (series, dataX, dataY, scaleX, scaleY, valToPosX, valToPosY, xOff, yOff, xDim, yDim, moveTo, lineTo, rect) => {
                        u.ctx.save();

                        xs.forEach((_, i) => {
                            if (!ys[i]) {
                                return;
                            }
                            const p = new Path2D();
                            rect(p, x + i * w + w / 2, y, w - margin, h - margin);
                            const b = norm ? ys[i] / norm : 1;
                            u.ctx.fillStyle = 'hsl(' + baselineColor + ' 100% ' + (75 - Math.trunc(b * 50)) + '%)';
                            u.ctx.fill(p);
                        });
                        u.ctx.restore();
                    },
                );
            };
        },
        selectionPlugin() {
            if (!this.selection) {
                return {};
            }
            const emitSelection = (s) => {
                this.$emit('select', s);
            };
            const init = (u) => {
                u.over.addEventListener('click', () => {
                    u.setSelect({ width: 0, height: 0 }, false);
                    emitSelection({});
                });
            };
            const ready = (u) => {
                const c = this.config;
                const sel = this.selection;
                if (!sel.x1 && !sel.x2 && !sel.y1 && !sel.y2) {
                    return;
                }
                const opts = { left: 0, width: 0, top: 0, height: 0 };
                opts.left = Math.max(u.valToPos(sel.x1 || c.ctx.from, 'x'), 0);
                opts.width = Math.min(u.valToPos(sel.x2 || c.ctx.to, 'x') - opts.left, u.bbox.width / devicePixelRatio);
                opts.top = (sel.y2 === '' ? 0 : u.valToPos(c.series.findIndex((s) => s.value === sel.y2) + 1, 'y')) + 1;
                opts.height =
                    (sel.y1 === '' ? u.bbox.height / devicePixelRatio : u.valToPos(c.series.findIndex((s) => s.value === sel.y1) + 1, 'y')) -
                    opts.top;
                this.select = opts;
                u.setSelect(opts, false);
            };
            const setSelect = (u) => {
                const c = this.config;
                const s = u.select;
                const rs = this.select;
                if ((!s.width && !s.height) || (s.left === rs.left && s.width === rs.width && s.top === rs.top && s.height === rs.height)) {
                    return;
                }
                const x1 = Math.trunc(u.posToVal(s.left, 'x'));
                const x2 = Math.trunc(u.posToVal(s.left + s.width, 'x'));
                let y1 = Math.trunc(u.posToVal(s.top + s.height, 'y'));
                let y2 = Math.trunc(u.posToVal(s.top, 'y'));
                y1 = y1 <= 0 ? '' : c.series[y1 - 1].value;
                const l = c.series.length;
                y2 = y2 >= l ? c.series[l - 1].value : c.series[y2].value;
                emitSelection({ x1, x2, y1, y2 });
            };

            return { hooks: { init, ready, setSelect } };
        },
    },
};
</script>

<style scoped>
.chart {
    position: relative;
}
.chart:deep(.u-select) {
    border: 1px dashed #ffeb3b;
    background-color: #ffeb3b80;
}

.title {
    font-size: 14px !important;
    font-weight: normal !important;
    text-align: center;
    line-height: 1.5em;
}

.legend {
    width: 120px;
    height: 10px;
    background: linear-gradient(to right, hsl(203, 100%, 75%), hsl(200 100% 25%));
    margin: 5px 10px 15px auto;
    display: flex;
}
.legend .item {
    font-size: 12px;
    margin-top: 10px;
    width: 100%;
}
.legend .item.min {
    text-align: left;
    margin-left: -10px;
}
.legend .item.avg {
    text-align: center;
}
.legend .item.max {
    text-align: right;
    margin-right: -10px;
}

.threshold {
    position: absolute;
    background-color: var(--background-color);
    border-top: 1px dashed var(--text-color);
    pointer-events: none;
}
.threshold .icon {
    position: absolute;
    right: -20px;
    top: -8px;
    pointer-events: auto;
}

.tooltip .item {
    display: flex;
    align-items: center;
    gap: 4px;
    padding-right: 50px;
    position: relative;
}
.tooltip .item.threshold {
    border-top: 1px dashed var(--text-color);
}
.tooltip .item .label {
    text-align: right;
}
.tooltip .item .value {
    width: 60px;
}
.tooltip .item .value .bar {
    height: 12px;
    background-color: hsl(200 100% 75%);
    font-size: 10px;
}
.tooltip .item .value .bar.error {
    background-color: hsl(0, 70%, 75%);
}
.tooltip .item .details {
    position: absolute;
    right: 0;
    top: -14px;
    text-align: right;
}
.tooltip .item .details .above {
    color: red;
}
.tooltip .item .details .below {
    color: green;
}

.loading {
    pointer-events: none;
}
</style>
