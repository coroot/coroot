<template>
    <div ref="container" class="chart">
        <div class="title">
            <slot name="title">
                <span v-html="config.title"></span>
            </slot>
            <a v-if="doc" :href="doc" target="_blank" class="ml-1"><v-icon small>mdi-information-outline</v-icon></a>

            <v-btn v-if="link" :to="link" x-small color="primary" class="ml-3">
                {{ link.title }}
            </v-btn>
        </div>

        <div ref="uplot" v-on-resize="redraw" class="chart" :class="{ loading: loading }">
            <div v-if="selection" ref="selection" class="selection">
                <div ref="selection-control" class="selection-control">
                    <v-btn-toggle v-if="selection.to > selection.from" :value="selection.mode" @change="setSelectionMode">
                        <v-btn small value="diff"><v-icon small class="mdi-flip-h">mdi-select-compare</v-icon></v-btn>
                        <v-btn small value="single"><v-icon small>mdi-magnify</v-icon></v-btn>
                        <v-btn small value=""><v-icon small>mdi-close</v-icon></v-btn>
                    </v-btn-toggle>
                </div>
                <div ref="selection-left" class="selection-left">
                    <span v-if="selection.mode === 'diff'" class="selection-title">baseline</span>
                </div>
                <div ref="selection-current" class="selection-current">
                    <span v-if="selection.mode === 'diff'" class="selection-title">comparison</span>
                </div>
            </div>

            <template v-if="!(selection && selection.to > selection.from && selection.mode)">
                <ChartAnnotations :ctx="config.ctx" :bbox="bbox" :annotations="annotations" />
            </template>
            <ChartIncidents :ctx="config.ctx" :bbox="bbox" :incidents="incidents" />
        </div>

        <ChartTooltip ref="tooltip" v-model="idx" :ctx="config.ctx" :incidents="incidents" class="tooltip">
            <div v-for="i in tooltip" class="item">
                <span class="marker" :style="{ backgroundColor: i.color }" />
                <span class="label">{{ i.label }}:</span>
                <span class="value">{{ i.value }}</span>
            </div>
        </ChartTooltip>

        <div v-if="legend" class="legend">
            <div
                v-for="l in legend"
                class="item"
                :style="{ opacity: l.hidden ? '0.5' : '1' }"
                @click="toggleSeries(l.label)"
                @mouseover="highlightSeries(l.label)"
                @mouseleave="highlightSeries(null)"
            >
                <span class="marker" :style="{ backgroundColor: l.color }"></span>
                <span class="label">{{ l.label }}</span>
            </div>
        </div>
    </div>
</template>

<script>
import 'uplot/dist/uPlot.min.css';
import uPlot from 'uplot';
import { palette } from '../utils/colors';
import convert from 'color-convert';
import ChartAnnotations from './ChartAnnotations';
import ChartIncidents from './ChartIncidents';
import ChartTooltip from './ChartTooltip';

const font = '12px Roboto, sans-serif';

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
        selection: Object,
        loading: Boolean,
        doc: String,
    },

    components: { ChartTooltip, ChartAnnotations, ChartIncidents },

    data() {
        return {
            ch: null,
            bbox: null,
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
        selection: {
            handler(s) {
                this.drawSelection(this.ch, s);
            },
            deep: true,
        },
        theme() {
            this.redraw();
        },
    },

    computed: {
        theme() {
            const dark = this.$vuetify.theme.dark;
            return {
                dark,
                text: dark ? 'rgba(255, 255, 255, 0.4)' : 'rgba(0,0,0,0.87)',
                grid: dark ? 'rgba(255, 255, 255, 0.3)' : 'rgba(0,0,0,0.07)',
            };
        },
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

            c.ctx.data = Array.from({ length: (c.ctx.to - c.ctx.from) / c.ctx.step + 1 }, (_, i) => c.ctx.from + i * c.ctx.step);

            const colors = {};
            c.series
                .filter((s) => s.color)
                .forEach((s, i) => {
                    if (!colors[s.color]) {
                        colors[s.color] = [];
                    }
                    colors[s.color].push(i);
                });

            c.series.forEach((s, i) => {
                s.stacked = s.stacked !== undefined ? s.stacked : c.stacked;
                if (s.color === 'black' && this.theme.dark) {
                    s.color = 'white';
                }
                if (colors[s.color] && colors[s.color].length > 1) {
                    const c = palette.get(s.color, 0);
                    const hsl = convert.hex.hsl(c);
                    const idx = colors[s.color].findIndex((ii) => ii === i);
                    hsl[2] = 70 - Math.trunc((idx * 30) / colors[s.color].length);
                    s.color = '#' + convert.hsl.hex(hsl);
                } else {
                    s.color = palette.get(s.color, i + (c.color_shift || 0));
                }
                s.fill = s.stacked || s.fill;
            });
            delete c.stacked;
            return c;
        },
        link() {
            const link = this.config.drill_down_link;
            if (!link) {
                return undefined;
            }
            const query = { ...this.$route.query, ...link.query };
            return { ...link, query };
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
            const ss = c.series.filter(this.isActive);
            const max = ss.reduce((p, c) => Math.max(p, ...c.data), null);
            const f = fmtVal(max, c.unit, 2);
            return ss.map((s) => ({ label: s.name, value: f(s.data[this.idx]), color: s.color }));
        },
        legend() {
            const c = this.config;
            if (c.hide_legend) {
                return null;
            }
            return c.series.map((s) => ({ label: s.name, color: s.color, hidden: !this.isActive(s) }));
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
                points: { show: false },
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
                data.unshift(s.data.map((v, i) => (a[i] += v)));
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
            if (!c.yzoom) {
                // fake series to show y == 0
                series.push({});
                data.push([0]);
            }
            const opts = {
                height: 150,
                padding: [10, 20, -10, 0],
                width: this.$refs.uplot.clientWidth,
                ms: 1,
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
                        space: 20,
                        font,
                        size: 60,
                        stroke: this.theme.text,
                        grid: { stroke: this.theme.grid },
                        ticks: { stroke: this.theme.grid },
                        values: (u, splits) => splits.map((v) => fmtVal(Math.max(...splits), c.unit)(v)),
                    },
                ],
                series: [{}, ...series],
                cursor: {
                    points: { show: false },
                    y: false,
                    drag: {
                        setScale: false,
                        click: () => {}, // allow `click` propagation for the selection buttons
                    },
                },
                legend: { show: false },
                plugins: [this.selectionPlugin(), this.$refs.tooltip.plugin()],
            };

            if (this.ch) {
                this.ch.destroy();
            }
            this.ch = new uPlot(opts, [c.ctx.data, ...data], this.$refs.uplot);
            this.ch.root.style.font = font;
            this.bbox = Object.entries(this.ch.bbox).reduce((o, e) => {
                o[e[0]] = e[1] / devicePixelRatio;
                return o;
            }, {});
        },
        drawSelection(u, s) {
            if (!u || !s) {
                return;
            }
            const opts = { left: 0, width: 0, height: 0 };
            if (s.to > s.from) {
                opts.left = u.valToPos(s.from, 'x');
                opts.width = u.valToPos(s.to, 'x') - opts.left;
                opts.height = u.bbox.height / devicePixelRatio;
            }
            u.setSelect(opts, false);
            this.setSelection(u, opts);
        },
        setSelection(u, s) {
            const empty = s.width === 0;
            this.$refs['selection'].style.display = empty || !this.selection.mode ? 'none' : 'block';
            if (empty) {
                return;
            }

            const diffMode = this.selection.mode === 'diff';

            const current = this.$refs['selection-current'];
            current.style.width = s.width + 'px';
            current.style.left = s.left + 'px';

            const left = this.$refs['selection-left'];
            left.style.display = diffMode ? 'block' : 'none';
            const lw = Math.min(s.width, s.left);
            left.style.width = lw + 'px';
            left.style.left = s.left - lw + 'px';

            const control = this.$refs['selection-control'];
            const cw = control.getBoundingClientRect().width;
            const cl = diffMode ? s.left - cw / 2 : s.left + (s.width - cw) / 2;
            control.style.left = cl + 'px';
        },
        setSelectionMode(m) {
            let { from, to } = this.selection;
            if (!m) {
                from = 0;
                to = 0;
            }
            this.emitSelection({ mode: m, from, to });
        },
        emitSelection(s) {
            const selection = { ...this.selection, ...s };
            const ctx = { from: this.config.ctx.from, to: this.config.ctx.to };
            this.$emit('select', { selection, ctx });
        },
        selectionPlugin() {
            if (!this.selection) {
                return {};
            }
            const init = (u) => {
                u.over.appendChild(this.$refs.selection);
            };
            const ready = (u) => {
                this.drawSelection(u, this.selection);
            };
            const setCursor = (u) => {
                if (u.select.width === 0) {
                    return;
                }
                this.setSelection(u, u.select);
            };
            const setSelect = (u) => {
                if (u.select.width === 0) {
                    return;
                }
                const sl = u.select.left;
                const sw = u.select.width;
                const from = Math.trunc(u.posToVal(sl, 'x'));
                const to = Math.trunc(u.posToVal(sl + sw, 'x'));
                if (this.selection.from === from || this.selection.to === to) {
                    return;
                }
                this.emitSelection({ from, to });
            };
            return { hooks: { init, ready, setCursor, setSelect } };
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
.chart {
    position: relative;
}

.chart:deep(.u-select) {
    background-color: var(--text-color);
    opacity: 0.12;
}

.title {
    font-size: 14px !important;
    font-weight: normal !important;
    text-align: center;
    line-height: 1.5em;
}

.selection {
    position: absolute;
    width: 100%;
    height: 100%;
}
.selection-control {
    position: relative;
    display: inline-block;
    transform: translateY(calc(-100% - 16px));
}
.selection-left {
    position: absolute;
    height: 100%;
    top: 0;
    border: 1px dashed var(--text-color-dimmed);
    border-right: none;
    border-bottom: none;
    color: rgba(0, 0, 0, 0.87);
}
.selection-current {
    position: absolute;
    height: 100%;
    top: 0;
    border: 1px solid var(--text-color-dimmed);
    border-bottom: none;
    color: rgba(0, 0, 0, 0.87);
}
.selection-title {
    position: absolute;
    top: -16px;
    font-style: italic;
    color: var(--text-color);
}
.selection-left .selection-title {
    right: 4px;
}
.selection-current .selection-title {
    left: 4px;
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

.loading {
    pointer-events: none;
}
</style>
