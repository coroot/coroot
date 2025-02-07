<template>
    <div v-if="show" :style="style">
        <div ref="name" @mouseenter="details = true" @mouseleave="details = false" @click="click">
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
                :limit="limit"
                :actions="actions"
            />
        </div>

        <v-tooltip v-if="details" :value="true" :activator="$refs.name" bottom transition="none" content-class="details">
            <v-card class="pa-2">
                <div class="font-weight-medium mb-1">{{ node.name }}</div>
                <template v-if="!!diff">
                    <div>baseline: {{ format(rates.base, '%') }} of total</div>
                    <div class="comparison">
                        comparison: {{ format(rates.comp, '%') }} of total
                        <template v-if="rates.diff !== 0">
                            (<span class="percent" :class="{ ok: rates.diff < 0 }">{{ format(rates.diff, '%', true) }}</span
                            >)
                        </template>
                    </div>
                </template>
                <template v-else>
                    <div>total: {{ format(node.total, unit) }} ({{ format(rates.root, '%') }})</div>
                    <div>self: {{ format(node.self, unit) }}</div>
                </template>
            </v-card>
        </v-tooltip>

        <v-menu v-if="actions" v-model="menu.show" absolute :position-x="menu.x" :position-y="menu.y" offset-y :open-on-click="false">
            <v-list dense class="pa-0" style="font-size: 14px">
                <v-list-item @click="emit()" dense class="px-2" style="min-height: 32px">
                    <v-icon small class="mr-1">mdi-magnify</v-icon>
                    Zoom in
                </v-list-item>
                <v-list-item v-for="a in actions" :to="a.to(node)" dense exact class="px-2" style="min-height: 32px">
                    <v-icon small class="mr-1">{{ a.icon }}</v-icon>
                    {{ a.title }}
                </v-list-item>
            </v-list>
        </v-menu>
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
        limit: Number,
        actions: Array,
    },

    data() {
        return {
            details: false,
            menu: {
                show: false,
                x: 0,
                y: 0,
            },
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
            return this.rates.root > (this.limit || 0);
        },
        style() {
            switch (this.zoom) {
                case false:
                    return { display: 'none' };
                case true:
                    return { display: 'block', width: '100%' };
                default:
                    return { display: 'block', width: Math.min(this.rates.parent, 100) + '%' };
            }
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
            if (this.node.color_by) {
                return palette.hash2(this.node.color_by);
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
        click(e) {
            if (this.actions) {
                this.menu.show = true;
                this.menu.x = e.clientX;
                this.menu.y = e.clientY;
            } else {
                this.emit();
            }
        },
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
    color: var(--text-light);
    filter: brightness(var(--brightness));
}
.name:hover {
    filter: brightness(calc(var(--brightness) + 20%));
}
.name.dimmed {
    filter: brightness(calc(var(--brightness) - 30%));
}
.children {
    display: flex;
    flex-wrap: nowrap;
    justify-content: flex-end;
}
.details {
    font-size: 12px;
}
.details .comparison .percent {
    font-weight: 600;
    color: var(--status-critical);
}
.details .comparison .percent.ok {
    color: var(--status-ok);
}
</style>
