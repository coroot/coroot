<template>
    <div>
        <div v-if="tooManyApplications" class="text-center red--text mt-5">
            Too many applications ({{ tooManyApplications }}) to render. Please choose a different category or namespace.
        </div>

        <div class="applications" v-on-resize="calc" @scroll="calc">
            <div v-for="apps in levels" class="level" style="z-index: 1; row-gap: 20px">
                <div v-for="a in apps" style="text-align: center">
                    <div :ref="a.id" class="app" @mouseenter="hi = a.id" @mouseleave="hi = null">
                        <div class="d-flex align-center">
                            <div class="flex-grow-1 name">
                                <router-link :to="{ name: 'overview', params: { view: 'applications', id: a.id }, query: $utils.contextQuery() }">
                                    {{ $utils.appId(a.id).name }}
                                </router-link>
                            </div>
                            <AppIcon :icon="a.icon" class="ml-1" />
                        </div>
                        <template v-if="a.issues">
                            <div class="mt-1 caption font-weight-medium">Issues:</div>
                            <div v-for="i in a.issues" class="issue">• <span v-html="i" /></div>
                        </template>
                    </div>
                </div>
            </div>
            <svg :style="{ zIndex: hi ? 2 : 0 }">
                <defs>
                    <template v-for="s in ['unknown', 'ok', 'warning', 'critical']">
                        <marker
                            :id="`marker-${s}`"
                            class="marker"
                            :class="s"
                            viewBox="0 0 10 10"
                            refX="10"
                            refY="5"
                            :markerWidth="10"
                            :markerHeight="10"
                            markerUnits="userSpaceOnUse"
                            orient="auto-start-reverse"
                        >
                            <path d="M 0 3 L 10 5 L 0 7 z" />
                        </marker>
                    </template>
                </defs>

                <template v-for="a in arrows">
                    <path v-if="a.dd" :d="a.dd" class="arrow" :class="a.status" />
                    <path :d="a.d" class="arrow" :class="a.status" :stroke-opacity="0.7" :marker-end="`url(#marker-${a.status})`" />
                </template>
            </svg>
            <template v-for="a in arrows">
                <div v-if="a.stats && hi" class="stats" :style="{ top: a.stats.y + 'px', left: a.stats.x + 'px', zIndex: 3 }">
                    <div v-for="i in a.stats.items">{{ i }}</div>
                </div>
            </template>
        </div>
    </div>
</template>

<script>
import { hash } from '../utils/colors';
import AppIcon from '@/components/AppIcon.vue';

function findBackLinks(index, a, discovered, finished, found) {
    if (!a) {
        return;
    }
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
    if (!a) {
        return;
    }
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
    },

    components: { AppIcon },

    data() {
        return {
            levels: [],
            arrows: [],
            hi: null,
            filter: new Set(this.applications.map((a) => a.id)),
            tooManyApplications: 0,
        };
    },

    mounted() {
        this.calc();
    },

    watch: {
        applications() {
            this.filter = new Set(this.applications.map((a) => a.id));
            this.calc();
        },
        selectedCategories() {
            this.calc();
        },
    },
    computed: {
        maxApplications() {
            return 1000;
        },
        hideLabels() {
            return this.levels.some((l) => l.length >= 15);
        },
    },
    methods: {
        setFilter(filter) {
            this.filter = filter;
            this.calc();
        },
        calc() {
            if (!this.applications) {
                return;
            }
            const index = new Map();
            this.applications.forEach((a) => {
                index.set(a.id, a);
            });
            this.tooManyApplications = 0;
            const filter = (a) => index.get(a.id) && this.filter.has(a.id);
            const applications = this.applications.filter(filter).map((a) => ({ ...a }));
            if (applications.length > this.maxApplications) {
                this.tooManyApplications = applications.length;
                this.levels = [];
                this.arrows = [];
                return;
            }
            applications.forEach((a) => {
                a.name = this.$utils.appId(a.id).name;
                a.levels = [];
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
                a.downstreams = a.downstreams.filter((d) => !backLinks.has(d.id + '->' + a.id));
            });

            const roots = applications.filter((a) => a.downstreams.length === 0);
            roots.forEach((a) => {
                calcLevel(index, a, 0, backLinks);
            });

            const depth = Math.max(...applications.map((a) => a.level));
            const levels = Array.from({ length: depth + 1 }, () => []);
            applications.forEach((a) => {
                levels[a.level].push(a);
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
                return { top: el.offsetTop, left: el.offsetLeft, width: el.offsetWidth, height: el.offsetHeight };
            };
            const index = new Map();
            applications.forEach((a) => {
                index.set(a.id, a);
            });
            const arrows = [];
            applications.forEach((app) => {
                app.upstreams.forEach((u) => {
                    const a = {
                        src: app.id,
                        dst: u.id,
                        status: u.status,
                    };
                    const s = getRect(a.src);
                    const d = getRect(a.dst);
                    if (!s || !d) {
                        return;
                    }
                    arrows.push(a);
                    a.x1 = s.left + s.width;
                    a.y1 = s.top + s.height / 2;
                    a.x2 = d.left;
                    a.y2 = d.top + d.height / 2;
                    const upstream = index.get(u.id);
                    if (upstream && Math.abs(upstream.level - app.level) > 1) {
                        a.y2 = d.top + 10 + (hash(app.id) % (d.height / 2 - 20));
                    }

                    if (a.x1 > a.x2) {
                        a.x1 = s.left;
                        a.x2 = d.left + d.width;
                    }
                    a.d = `M${a.x1},${a.y1} L${a.x2},${a.y2}`;

                    if (u.stats && u.stats.length) {
                        const maxLength = u.stats.reduce((max, str) => Math.max(max, str.length), 0);
                        let x = (a.x2 + a.x1) / 2 - (maxLength * 5) / 2;
                        const y = (a.y2 + a.y1) / 2 - (u.stats.length * 12) / 2;
                        a.stats = { x, y, items: u.stats };
                    }
                });
            });
            this.arrows = arrows;
        },
    },
};
</script>

<style scoped>
.applications {
    position: relative;
    display: flex;
    padding: 10px 0;
    overflow-x: auto;
}
.level {
    min-width: 120px;
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    /*row-gap: 32px;*/
    justify-content: space-around;
}
.app {
    min-width: 150px;
    max-width: 100%;
    border: 1px solid #bdbdbd;
    border-radius: 3px;
    padding: 4px 8px;
    background-color: var(--background-color);
    display: inline-flex;
    flex-direction: column;
    line-height: 1.1;
    text-align: left;
}
.app.hidden {
    visibility: hidden;
}

.app.selected {
    border: 1px solid var(--text-color);
    background-color: var(--background-color-hi);
}
.name {
    white-space: nowrap;
    display: inline-block;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
}

.label {
    margin-left: 14px;
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
    fill: none;
}

.arrow.unknown {
    stroke: var(--status-unknown);
}
.arrow.ok {
    stroke: var(--status-ok);
    stroke-width: 1.5;
}
.arrow.warning {
    stroke: var(--status-warning);
    stroke-width: 1.5;
}
.arrow.critical {
    stroke: var(--status-critical);
    stroke-width: 1.5;
}
.marker.unknown {
    fill: var(--status-unknown);
}
.marker.ok {
    fill: var(--status-ok);
}
.marker.warning {
    fill: var(--status-warning);
}
.marker.critical {
    fill: var(--status-critical);
}

.stats {
    position: absolute;
    font-size: 12px;
    line-height: 12px;
    background-color: var(--background-color-hi);
    padding: 4px;
    border-radius: 2px;
    pointer-events: none;
}

.issue {
    font-size: 0.75rem !important;
    font-weight: 400;
    letter-spacing: 0.0333333333em !important;
    line-height: 1rem;
}
</style>
