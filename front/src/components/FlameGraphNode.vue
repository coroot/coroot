<template>
    <div v-if="show" :style="{ width }">
        <div ref="name" @mouseenter="details = true" @mouseleave="details = false" @click="emit()">
            <div class="name" :class="{ dimmed: zoom && zoomed }" :style="{ backgroundColor: color }">
                {{ node.name }}
                <template v-if="!!diff">
                    <template v-if="rates.diff"> ({{ format(rates.diff, '%', true) }}) </template>
                </template>
                <template v-else> ({{ format(node.total, unit) }}, {{ format(rates.root, '%') }}) </template>
            </div>
        </div>

        <div class="children">
            <FlameGraphNode
                v-for="n in node.children"
                :node="n"
                :parent="node"
                :root="root"
                :zoom="zoomed ? zoomed === n.name : undefined"
                @zoom="emit(n)"
                :search="search"
                :diff="diff"
                :unit="unit"
            />
        </div>

        <v-tooltip v-if="details" :value="true" :activator="$refs.name" bottom transition="none" content-class="details">
            <v-card class="pa-2">
                <div class="font-weight-medium mb-1">{{ node.name }}</div>
                <template v-if="!!diff">
                    <div>baseline: {{ format(rates.base, '%') }} of total</div>
                    <div>
                        comparison: {{ format(rates.comp, '%') }} of total (<span
                            :style="{ color: rates.diff > 0 ? 'red' : 'green' }"
                            class="font-weight-medium"
                            >{{ format(rates.diff, '%', true) }}</span
                        >)
                    </div>
                </template>
                <template v-else>
                    <div>total: {{ format(node.total, unit) }} ({{ format(rates.root, '%') }})</div>
                    <div>self: {{ format(node.self, unit) }}</div>
                </template>
            </v-card>
        </v-tooltip>
    </div>
</template>

<script>
import { palette } from '../utils/colors';

export default {
    name: 'FlameGraphNode',

    props: {
        node: Object,
        parent: Object,
        root: Object,
        zoom: Boolean,
        search: String,
        diff: Number,
        unit: String,
    },

    data() {
        return {
            details: false,
            zoomed: '',
        };
    },

    computed: {
        rates() {
            const r = {
                root: (this.node.total / this.root.total) * 100,
                parent: (this.node.total / this.parent.total) * 100,
                base: ((this.node.total - this.node.comp) / (this.root.total - this.root.comp)) * 100,
                comp: (this.node.comp / this.root.comp) * 100,
            };
            r.diff = r.comp - r.base;
            return r;
        },
        show() {
            return this.rates.root > 0.5;
        },
        width() {
            if (this.zoom === false) {
                return '0';
            }
            if (this.zoom === true) {
                return '100%';
            }
            return this.rates.parent + '%';
        },
        color() {
            if (!!this.search && !this.node.name.toLowerCase().includes(this.search.toLowerCase())) {
                return palette.get('grey-lighten3');
            }
            if (this.diff) {
                let p = this.rates.diff / this.diff;
                p = p < 0 ? Math.max(p, -1) : Math.min(p, 1);
                p = (p * 80).toFixed(0);
                return p < 0 ? `hsl(120, ${-p}%, 70%)` : `hsl(0, ${p}%, 70%)`;
            }
            let name = this.node.name;
            const i = name.lastIndexOf('/');
            if (i > 0) {
                name = name.substr(0, i);
            }
            return palette.hash2(name);
        },
    },

    watch: {
        zoom(v) {
            if (!v) {
                this.zoomed = '';
            }
        },
    },

    methods: {
        emit(n) {
            this.zoomed = n ? n.name : '';
            this.$emit('zoom');
        },
        format(v, unit, sign) {
            const s = sign && v > 0 ? '+' : '';
            const va = Math.abs(v);
            if (unit === '%') {
                let d = 2;
                if (va > 1) {
                    d = 1;
                }
                if (va > 10) {
                    d = 0;
                }
                return s + v.toFixed(d) + '%';
            }
            if (unit === 'nanoseconds') {
                unit = 'ns';
                if (va > 1e3) {
                    v /= 1000;
                    unit = 'μs';
                }
                if (va > 1e6) {
                    v /= 1000;
                    unit = 'ms';
                }
                if (va > 1e9) {
                    v /= 1000;
                    unit = 's';
                }
                if (va > 60e9) {
                    v /= 60;
                    unit = 'min';
                }
                return s + v.toFixed(0) + ' ' + unit;
            }
            if (unit === 'bytes') {
                unit = 'B';
                if (va > 1e3) {
                    v /= 1000;
                    unit = 'KB';
                }
                if (va > 1e6) {
                    v /= 1000;
                    unit = 'MB';
                }
                if (va > 1e9) {
                    v /= 1000;
                    unit = 'GB';
                }
                return s + v.toFixed(1) + ' ' + unit;
            }
            if (va > 1e3) {
                v /= 1000;
                unit = 'K';
            }
            if (va > 1e6) {
                v /= 1000;
                unit = 'M';
            }
            if (va > 1e9) {
                v /= 1000;
                unit = 'G';
            }
            return s + v.toFixed(1) + ' ' + unit;
        },
    },
};
</script>

<style scoped>
.name {
    cursor: pointer;
    font-size: 12px;
    white-space: nowrap;
    overflow: hidden;
    text-indent: 4px;
    border: 0.2px solid rgba(255, 255, 255, 0.5);
    padding: 2px 0;
}
.name:hover {
    filter: brightness(120%);
}
.name.dimmed {
    filter: opacity(50%);
}
.children {
    display: flex;
    flex-wrap: nowrap;
    justify-content: flex-end;
}
.details {
    opacity: 1;
    padding: 0;
    font-size: 12px;
}
</style>
