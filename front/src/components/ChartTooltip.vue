<template>
    <div ref="tooltip" class="tooltip">
        <div class="time">{{ time }}</div>
        <slot></slot>
        <div v-if="incident" class="incident">
            <span class="label">incident</span>: {{ incident.from }} - {{ incident.to || 'in progress' }} ({{ incident.dur }})
        </div>
    </div>
</template>

<script>
const tsFormat = '{MMM} {DD}, {HH}:{mm}:{ss}';
const tsFormatShort = '{MMM} {DD}, {HH}:{mm}';

export default {
    props: {
        ctx: Object,
        incidents: Array,
    },
    data() {
        return {
            idx: null,
            mousedown: false,
        };
    },
    computed: {
        ts() {
            return this.ctx.data[this.idx];
        },
        time() {
            return this.$format.date(this.ts, tsFormat);
        },
        incident() {
            const incident = (this.incidents || []).find((o) => o.x1 <= this.ts && this.ts <= o.x2);
            if (!incident) {
                return null;
            }
            const long = incident.x2 - incident.x1 > 3600000;
            const fmt = long ? tsFormatShort : tsFormat;
            const precision = long ? 'm' : 's';
            return {
                from: this.$format.date(incident.x1, fmt),
                to: incident.x2 < this.ctx.to && this.$format.date(incident.x2, fmt),
                dur: this.$format.duration(incident.x2 - incident.x1 + this.ctx.step, precision),
            };
        },
    },
    methods: {
        plugin() {
            const init = (u) => {
                const t = this.$refs.tooltip;
                u.over.appendChild(t);
                u.over.addEventListener('mouseleave', () => {
                    t.style.display = 'none';
                    this.mousedown = false;
                });
                u.over.addEventListener('mousedown', () => {
                    t.style.display = 'none';
                    this.mousedown = true;
                });
                u.over.addEventListener('mouseup', () => {
                    this.mousedown = false;
                });
            };
            const setCursor = (u) => {
                const { left, top, idx } = u.cursor;
                if (idx === null) {
                    return;
                }
                this.idx = idx;
                this.$emit('input', idx);
                const t = this.$refs.tooltip;
                const l = left - (left >= u.over.clientWidth / 2 ? t.clientWidth + 5 : -5);
                t.style.transform = 'translate(' + l + 'px, ' + top + 'px)';
                if (!this.mousedown) {
                    t.style.display = 'block';
                }
            };
            return { hooks: { init, setCursor } };
        },
    },
};
</script>

<style scoped>
.tooltip {
    display: none;
    position: absolute;
    z-index: 2;
    background-color: var(--tooltip-color);
    padding: 8px;
    border: 1px solid rgba(0, 0, 0, 0.2);
    border-radius: 4px;
    pointer-events: none;
    font-size: 12px;
    opacity: 90%;
}
.tooltip .time {
    text-align: center;
    margin-bottom: 8px;
}
.tooltip .incident {
    margin-top: 10px;
    padding-top: 5px;
    border-top: 1px solid black;
}
.tooltip .incident .label {
    color: hsl(4, 90%, 58%);
    font-weight: 600;
}
</style>
