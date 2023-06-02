<template>
    <div v-on-resize="calcArrows" class="map">
        <div class="column" :style="{rowGap: columnRowGap(map.clients)}">
            <div v-for="app in map.clients" class="client" :ref="app.id"
                 :class="{hi: highlighted.clients.has(app.id)}"
                 @mouseenter="focus('client', app.id)" @mouseleave="unfocus"
            >
                <div>
                    <router-link :to="{name: 'application', params: {id: app.id}, query: $utils.contextQuery()}" class="name">
                        <AppHealth :app="app"/>
                    </router-link>
                    <Labels v-if="!hideLabels(map.clients)" :labels="app.labels" class="d-none d-sm-block ml-4" />
                </div>
            </div>
        </div>

        <div class="column">
            <div v-if="map.application" class="app" :ref="map.application.id">
                <div>
                    <span class="name">
                        <AppHealth :app="map.application"/>
                    </span>
                    <Labels :labels="map.application.labels" class="d-none d-sm-block ml-4" />
                </div>
                <div v-if="map.instances && map.instances.length" class="instances">
                    <div v-for="i in map.instances" class="instance" :ref="'instance:'+i.id"
                         :class="{hi: highlighted.instances.has(i.id)}"
                         @mouseenter="focus('instance', i.id)" @mouseleave="unfocus"
                    >
                        <span class="name">
                            {{i.id}}
                            <v-icon v-if="i.labels && i.labels['role'] === 'primary'" small color="rgba(0,0,0,0.87)">mdi-database-edit-outline</v-icon>
                            <v-icon v-if="i.labels && i.labels['role'] === 'replica'" small color="grey">mdi-database-import-outline</v-icon>
                        </span>
                        <Labels :labels="i.labels" class="d-none d-sm-block" />
                    </div>
                </div>
            </div>
        </div>

        <div class="column" :style="{rowGap: columnRowGap(map.dependencies)}">
            <div v-for="app in map.dependencies" class="dependency" :ref="app.id"
                 :class="{hi: highlighted.dependencies.has(app.id)}"
                 @mouseenter="focus('dependency', app.id)" @mouseleave="unfocus"
            >
                <div>
                    <router-link :to="{name: 'application', params: {id: app.id}, query: $utils.contextQuery()}" class="name">
                        <AppHealth :app="app"/>
                    </router-link>
                    <Labels v-if="!hideLabels(map.dependencies)" :labels="app.labels" class="d-none d-sm-block ml-4" />
                </div>
            </div>
        </div>

        <svg>
            <defs>
                <marker :id="m" v-for="m in ['marker', 'markerhi', 'markerlo']"
                        viewBox="0 0 10 10" refX="10" refY="5" markerWidth="10" markerHeight="10" markerUnits="userSpaceOnUse" orient="auto-start-reverse"
                >
                    <path d="M 0 3 L 10 5 L 0 7 z" />
                </marker>
            </defs>
            <template v-for="a in arrows">
                <path v-if="a.dd && a.hi(focused)==='hi' && a.w(arrows, focused)" :d="a.dd(arrows, focused)" class="arrow" :class="a.status" stroke="none" />
                <path :d="a.d" class="arrow" :class="[a.status, a.hi(focused)]" fill-opacity="0"
                      :marker-start="a.markerStart ? `url(#marker${a.hi(focused)})` : ''" :marker-end="a.markerEnd ? `url(#marker${a.hi(focused)})`: ''"
                />
            </template>
        </svg>
        <template v-for="a in arrows">
            <div v-if="a.stats && a.hi(focused)==='hi'" class="stats" :style="{top: a.stats.y+'px', left: a.stats.x+'px'}">
                <div v-for="i in a.stats.items">{{i}}</div>
            </div>
        </template>
    </div>
</template>

<script>
import Labels from './Labels';
import AppHealth from "./AppHealth";

export default {
    props: {
        map: Object,
    },

    components: {Labels, AppHealth},

    data() {
        return {
            arrows: [],
            focused: {},
        };
    },

    mounted() {
        requestAnimationFrame(this.calcArrows);
    },

    watch: {
        map() {
            requestAnimationFrame(this.calcArrows);
            this.unfocus();
        },
    },

    computed: {
        highlighted() {
            const res = {
                clients: new Set(),
                dependencies: new Set(),
                instances: new Set(),
            };
            const instances = this.map.instances || [];
            if (this.focused.instance) {
                res.instances.add(this.focused.instance);
                const instance = instances.find((i) => i.id === this.focused.instance);
                if (!instance) {
                    return res;
                }
                (instance.clients || []).forEach((a) => {
                    res.clients.add(a.id);
                });
                (instance.dependencies || []).forEach((a) => {
                    res.dependencies.add(a.id);
                });
                (instance.internal_links || []).forEach(l => {
                    res.instances.add(l.id);
                });
                instances.forEach((i) => {
                    if (i.internal_links && i.internal_links.find(l => l.id === this.focused.instance)) {
                        res.instances.add(i.id);
                    }
                })
            }
            if (this.focused.client) {
                res.clients.add(this.focused.client);
                instances.forEach((i) => {
                    if (i.clients && i.clients.find((a) => a.id === this.focused.client)) {
                        res.instances.add(i.id);
                    }
                })
            }
            if (this.focused.dependency) {
                res.dependencies.add(this.focused.dependency);
                instances.forEach((i) => {
                    if (i.dependencies && i.dependencies.find((a) => a.id === this.focused.dependency)) {
                        res.instances.add(i.id);
                    }
                })
            }
            return res;
        },
        links() {
            const links = [];
            (this.map.instances || []).forEach((i) => {
                const me = (focused) => focused.instance && focused.instance === i.id;
                const lo = (focused) => Object.keys(focused).length ? 'lo': '';
                (i.clients || []).forEach((a) => {
                    const from = a.id;
                    const to = 'instance:'+i.id;
                    const hi = (focused) => (me(focused) || focused.client && focused.client === from) ? 'hi' : lo(focused);
                    links.push({from, to, status: a.status, stats: a.stats, weight: a.weight, direction: a.direction, hi});
                });
                (i.dependencies || []).forEach((a) => {
                    const from = 'instance:'+i.id;
                    const to = a.id;
                    const hi = (focused) => (me(focused) || focused.dependency && focused.dependency === to) ? 'hi' : lo(focused);
                    links.push({from, to, status: a.status, stats: a.stats, weight: a.weight, direction: a.direction, hi});
                });
                (i.internal_links || []).forEach((l) => {
                    const from = 'instance:'+i.id;
                    const to = 'instance:'+l.id;
                    const hi = (focused) => (me(focused) || focused.instance && focused.instance === l.id) ? 'hi' : lo(focused);
                    links.push({from, to, status: l.status, direction: l.direction, hi, internal: true});
                });
            });
            return links;
        },
    },

    methods: {
        hideLabels(items) {
            return items && items.length > 15;
        },
        columnRowGap(items) {
            return (items && items.length > 15 ? 4 : 16) + 'px';
        },
        focus(type, id) {
            this.focused = {};
            if (!this.map.instances) {
                return;
            }
            this.focused[type] = id;
        },
        unfocus() {
            this.focused = {};
        },
        getRect(ref) {
            const el = this.$refs[ref] && (this.$refs[ref][0] || this.$refs[ref]);
            if (!el) {
                return null;
            }
            return {top: el.offsetTop, left: el.offsetLeft, width: el.offsetWidth, height: el.offsetHeight};
        },
        calcArrows() {
            const arrows = [];
            this.links.forEach((l) => {
                const src = this.getRect(l.from);
                const dst = this.getRect(l.to);
                if (!src || !dst) {
                    return;
                }
                const a = {
                    hi: l.hi,
                    status: l.status,
                    _w: l.weight || 0,
                    markerStart: l.direction === 'from' || l.direction === 'both',
                    markerEnd: l.direction === 'to' || l.direction === 'both',
                };
                arrows.push(a);
                if (l.internal) {
                    const x1 = src.left + src.width;
                    const y1 = src.top + src.height / 2;
                    const x2 = dst.left + dst.width;
                    const y2 = dst.top + dst.height / 2;
                    const r = Math.abs(y2 - y1);
                    const rx = r;
                    const ry = r;
                    const sweep = y2 > y1 ? 1 : 0;
                    a.d = `M${x1},${y1} A${rx},${ry} 0,0,${sweep} ${x2},${y2}`;
                } else {
                    const x1 = src.left + src.width;
                    const y1 = src.top + src.height / 2;
                    const x2 = dst.left;
                    const y2 = dst.top + dst.height / 2;
                    a.d = `M${x1},${y1} L${x2},${y2}`;
                    a.w = (as, hi) => 3 * a._w / Math.max(...as.filter((a) => a.hi(hi) === 'hi').map((a) => a._w));
                    a.r = (as, hi) => a.w(as, hi)/2 + ((y2 - y1)**2 + (x2 - x1)**2)/(8*a.w(as, hi));
                    a.dd = (as, hi) => `M${x1},${y1} A${a.r(as, hi)},${a.r(as, hi)} 0,0,0 ${x2},${y2} A${a.r(as, hi)},${a.r(as, hi)} 0,0,0 ${x1},${y1}`;
                    if (l.stats && l.stats.length) {
                        a.stats = {x: (x2+x1)/2-20, y: (y2+y1)/2-(l.stats.length*12)/2, items: l.stats}
                    }
                }
            });
            this.arrows = arrows;
        },
    },
};
</script>

<style scoped>
.map {
    display: flex;
    justify-content: space-between;
    line-height: 1.1;
    position: relative;
    gap: 16px;
    overflow-x: auto;
    padding: 10px 0;
}
.column {
    flex-basis: 10%; /* to keep some space if no clients or no dependencies */
    display: flex;
    flex-direction: column;
    row-gap: 16px;
    align-self: center;
}
.app, .client, .dependency {
    max-width: 300px;
    border-radius: 3px;
    border: 1px solid #BDBDBD;
    white-space: nowrap;
    padding: 4px 8px;
}
.instances {
    padding: 8px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
}
.instance {
    border-radius: 3px;
    border: 1px solid #BDBDBD;
    white-space: nowrap;
    padding: 4px 8px;
    max-width: 12rem;
}
.name {
    white-space: nowrap;
    display: inline-block;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
}
.hi {
    border: 1px solid rgba(0,0,0,0.87);
    background-color: #cbe9fc;
}
svg {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none; /* to allow interactions with html below */
    overflow: visible;
}
.arrow {
    stroke-width: 1;
    stroke-opacity: 0.7;
}
.arrow.hi {
    stroke-opacity: 1;
}
.arrow.lo {
    stroke-opacity: 0.1;
}
.arrow.ok {
    stroke: green;
    fill: green;
}
.arrow.warning {
    stroke: red;
    stroke-dasharray: 4;
    fill: red;
}
.arrow.unknown {
    stroke: lightgray;
    stroke-dasharray: 4;
    fill: lightgray;
}
#marker path {
    fill-opacity: 0.3;
}
#markerlo path {
    fill-opacity: 0.1;
}
#markerhi path {
    fill-opacity: 1.0;
}
.stats {
    position: absolute;
    font-size: 12px;
    line-height: 12px;
    background-color: #EEEEEE;
    padding: 2px;
    border-radius: 2px;
}
</style>
