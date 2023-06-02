<template>
    <div>
        <div v-if="filters" class="filters mb-3">
            <v-spacer v-if="$vuetify.breakpoint.mdAndUp"></v-spacer>
            <v-checkbox v-for="f in filters" :key="f.name" v-model="f.value" :label="f.name" class="filter" color="green" hide-details @click="calc" />
        </div>
        <div class="applications" v-on-resize="calc" @scroll="calc">
            <div v-for="apps in levels" class="level" style="z-index: 1" :style="{rowGap: 200 / apps.length + 'px', maxWidth: 100 / levels.length + '%' }">
                <div v-for="a in apps" style="text-align: center">
                    <span :ref="a.id" class="app" :class="a.hi(hi) ? 'selected' : ''" @mouseenter="hi = a.id" @mouseleave="hi = null">
                        <router-link :to="{name: 'application', params: {id: a.id}, query: $utils.contextQuery()}" class="name">
                            <AppHealth :app="a"/>
                        </router-link>
                        <Labels v-if="!hideLabels" :labels="a.labels" class="d-none d-sm-block ml-4" />
                    </span>
                </div>
            </div>
            <svg :style="{zIndex: hi ? 2 : 0}">
                <defs>
                    <marker id="arrow" viewBox="0 0 10 10" refX="10" refY="5" :markerWidth="10" :markerHeight="10" markerUnits="userSpaceOnUse" orient="auto-start-reverse">
                        <path d="M 0 3 L 10 5 L 0 7 z" />
                    </marker>
                </defs>

                <template v-for="a in arrows">
                    <path v-if="a.hi(hi) && a.w(arrows, hi)" :d="a.dd(arrows, hi)" class="arrow" :class="a.status" />
                    <path :d="a.d" class="arrow" :class="a.status" :stroke-opacity="a.hi(hi) ? 1 : 0.3" marker-end="url(#arrow)"/>
                </template>
            </svg>
            <template v-for="a in arrows">
                <div v-if="a.stats && a.hi(hi)" class="stats" :style="{top: a.stats.y+'px', left: a.stats.x+'px', zIndex: 3}">
                    <div v-for="i in a.stats.items">{{i}}</div>
                </div>
            </template>
        </div>
    </div>
</template>

<script>
import Labels from "./Labels";
import AppHealth from "./AppHealth";

function findBackLinks(index, a, discovered, finished, found) {
    discovered.add(a.id);
    for (const u of a.upstreams) {
        if (discovered.has(u.id)) {
            found.add(a.id + '->' + u.id);
            continue;
        }
        if (!finished.has(u.id)) {
            findBackLinks(index, index.get(u.id), discovered, finished, found);
        }
    }
    discovered.delete(a.id);
    finished.add(a.id);
}

function calcLevel(index, a, level, backLinks) {
    if (a.level === undefined || level > a.level) {
        a.level = level;
    }
    for (const u of a.upstreams) {
        const l = a.id + '->' + u.id;
        if (backLinks.has(l)) {
            continue;
        }
        calcLevel(index, index.get(u.id), level + 1, backLinks);
    }
}

export default {
    props: {
        applications: Array,
        openApp: Function,
        noFilter: Boolean,
    },

    components: {AppHealth, Labels},

    data() {
        return {
            levels: [],
            arrows: [],
            hi: null,
            filters: [],
        };
    },

    mounted() {
        this.calcFilters();
        this.calc();
    },

    watch: {
        applications() {
            this.calcFilters();
            this.calc();
        },
    },
    computed : {
        hideLabels() {
            return this.levels.some((l) => l.length >= 15);
        },
    },
    methods: {
        calcFilters() {
            const applications = this.applications;
            if (!applications || !applications.length) {
                this.filters = [];
                return;
            }
            let filters = new Set();
            for (const a of applications) {
                filters.add(a.category);
            }
            if (filters.size === 0) {
                this.filters = [];
                return;
            }
            filters = Array.from(filters).map((f) => ({name: f, value: false}));
            filters.sort((a, b) => a.name.localeCompare(b.name));
            filters.forEach((f) => {
                const fff = this.filters.find((ff) => ff.name === f.name);
                fff && fff.value && (f.value = true);
            })
            if (!filters.some((f) => f.value)) {
                if (this.noFilter) {
                    filters.forEach((f) => {
                        f.value = true;
                    })
                } else {
                    filters[0].value = true;
                }
            }
            this.filters = filters;
        },
        calc() {
            let applications = Array.from(this.applications || []);
            const index = new Map();
            applications.forEach((a) => {
                index.set(a.id, a);
            });
            let filter = () => true;
            if (this.filters.length > 0) {
                const filters = new Map();
                for (const f of this.filters) {
                    filters.set(f.name, f.value);
                }
                filter = (a) => index.get(a.id) && filters.get(index.get(a.id).category);
            }
            applications = applications.filter(filter);
            applications.forEach((a) => {
                a.name = this.$utils.appId(a.id).name;
                a.level = 0;
                a.upstreams = a.upstreams.filter(filter);
                a.upstreams.sort((u1, u2) => u1.id.localeCompare(u2.id));
                a.downstreams = a.downstreams.filter(filter);
                a.hi = (hi) => Array.of(a, ...a.upstreams, ...a.downstreams).some((aa) => aa.id === hi);
            });
            applications.sort((a, b) => a.name.localeCompare(b.name));
            this.calcLevels(applications);
            requestAnimationFrame(() => this.calcArrows(applications));
        },
        calcLevels(applications) {
            if (applications.length === 0) {
                this.levels = [];
                return;
            }
            const index = new Map();
            applications.forEach((a) => {
                index.set(a.id, a);
            });
            const backLinks = new Set();
            applications.forEach((a) => {
                findBackLinks(index, a, new Set(), new Set(), backLinks);
            });
            applications.forEach((a) => {
                a.downstreams = a.downstreams.filter((d) => !backLinks.has(d.id+'->'+a.id))
            });

            const roots = applications.filter((a) => a.downstreams.length === 0);
            roots.forEach((a) => {
                calcLevel(index, a, 0, backLinks);
            });

            const depth = Math.max(...applications.map((a) => a.level));
            const levels = Array.from({length: depth+1}, () => []);
            applications.forEach((a) => {
                let l = a.level;
                if (a.upstreams.length === 0 && a.downstreams.length === 0) {
                    l = depth;
                } else if (a.downstreams.length === 0) {
                    l = 0;
                } else if (a.upstreams.length === 0) {
                    l = depth;
                }
                levels[l].push(a);
            });
            this.levels = levels;
        },
        calcArrows(applications) {
            if (!applications.length) {
                this.arrows = [];
                return;
            }
            const getRect = (ref) => {
                const el = this.$refs[ref] && (this.$refs[ref][0] || this.$refs[ref]);
                if (!el) {
                    return null;
                }
                return {top: el.offsetTop, left: el.offsetLeft, width: el.offsetWidth, height: el.offsetHeight};
            };
            const arrows = [];
            applications.forEach((app) => {
                app.upstreams.forEach((u) => {
                    const a = {
                        src: app.id,
                        dst: u.id,
                        status: u.status,
                        hi: (hi) => a.src === hi || a.dst === hi,
                        _w: u.weight || 0,
                    };
                    const s = getRect(a.src)
                    const d = getRect(a.dst)
                    if (!s || !d) {
                        return;
                    }
                    arrows.push(a);

                    let x1 = s.left + s.width;
                    let y1 = s.top  + s.height / 2;
                    let x2 = d.left;
                    let y2 = d.top + d.height / 2;
                    if (x1 > x2) {
                        x1 = s.left;
                        x2 = d.left + d.width;
                    }
                    a.d = `M${x1},${y1} L${x2},${y2}`;

                    a.w = (as, hi) => 3 * a._w / Math.max(...as.filter((a) => a.hi(hi)).map((a) => a._w));
                    a.r = (as, hi) => a.w(as, hi)/2 + ((y2 - y1)**2 + (x2 - x1)**2)/(8*a.w(as, hi));
                    a.dd = (as, hi) => `M${x1},${y1} A${a.r(as, hi)},${a.r(as, hi)} 0,0,0 ${x2},${y2} A${a.r(as, hi)},${a.r(as, hi)} 0,0,0 ${x1},${y1}`;

                    if (u.stats && u.stats.length) {
                        a.stats = {x: (x2+x1)/2-20, y: (y2+y1)/2-(u.stats.length*12)/2, items: u.stats};
                    }
                });
            });
            this.arrows = arrows;
        },
    },
};
</script>

<style scoped>
.filters {
    display: flex;
    flex-wrap: wrap;
}
.filter{
    margin: 0 8px;
}
.applications {
    position: relative;
    display: flex;
    padding: 10px 0;
    overflow-x: auto;
    gap: 16px;
}
.level {
    min-width: 120px;
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    justify-content: space-around;
    /*row-gap: 32px;*/
}
.app {
    max-width: 100%;
    border: 1px solid #BDBDBD;
    border-radius: 3px;
    white-space: nowrap;
    padding: 4px 8px;
    background-color: white;
    display: inline-flex;
    flex-direction: column;
    line-height: 1.1;
    text-align: left;
}
.app.selected {
    border: 1px solid rgba(0,0,0,0.87);
    background-color: #cbe9fc;
}
.name {
    white-space: nowrap;
    display: inline-block;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
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
    stroke: gray;
    stroke-dasharray: 4;
    fill: gray;
}
.stats {
    position: absolute;
    font-size: 12px;
    line-height: 12px;
    background-color: #EEEEEE;
    padding: 2px;
    border-radius: 2px;
    text-align: right;
}
</style>
