<template>
    <div>
        <v-progress-linear indeterminate v-if="loading" color="green" />

        <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
            {{ error }}
        </v-alert>

        <NoData v-else-if="!loading && !applications.length" />

        <ApplicationFilter v-else :applications="applications" :autoSelectNamespaceThreshold="maxApplications" @filter="setFilter" class="my-4" />

        <div v-if="tooManyApplications" class="text-center red--text mt-5">
            Too many applications ({{ tooManyApplications }}) to render. Please choose a different category or namespace.
        </div>

        <div class="applications" v-on-resize="calc" @scroll="calc">
            <div
                v-for="apps in levels"
                class="level"
                style="z-index: 1"
                :style="{ rowGap: 200 / apps.length + 'px', maxWidth: 100 / levels.length + '%' }"
            >
                <div v-for="a in apps" style="text-align: center">
                    <div :ref="a.id" class="app" :class="{ selected: a.hi(hi) }" @mouseenter="hi = a.id" @mouseleave="hi = null">
                        <div class="d-flex">
                            <div class="flex-grow-1 name">
                                <router-link :to="{ name: 'overview', params: { view: 'applications', id: a.id }, query: $utils.contextQuery() }">
                                    <AppHealth :app="a" />
                                </router-link>
                            </div>
                            <div>
                                <AppPreferences :app="a" :categories="categories" />
                            </div>
                        </div>
                        <Labels v-if="!hideLabels" :labels="a.labels" class="d-none d-sm-block label" />
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
                    <path :d="a.d" class="arrow" :class="a.status" :stroke-opacity="a.hi ? 1 : 0.7" :marker-end="`url(#marker-${a.status})`" />
                </template>
            </svg>
            <template v-for="a in arrows">
                <div v-if="a.stats && a.hi" class="stats" :style="{ top: a.stats.y + 'px', left: a.stats.x + 'px', zIndex: 3 }">
                    <div v-for="i in a.stats.items">{{ i }}</div>
                </div>
            </template>
        </div>
    </div>
</template>

<script>
import Labels from '@/components/Labels.vue';
import AppHealth from '@/components/AppHealth.vue';
import ApplicationFilter from '@/components/ApplicationFilter.vue';
import AppPreferences from '@/components/AppPreferences.vue';
import NoData from '@/components/NoData.vue';

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
    components: { NoData, AppPreferences, ApplicationFilter, AppHealth, Labels },

    data() {
        return {
            applications: [],
            categories: [],
            loading: false,
            error: '',
            levels: [],
            arrows: [],
            hi: null,
            filter: new Set(),
            tooManyApplications: 0,
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
        this.calc();
    },

    watch: {
        applications() {
            this.calc();
        },
        hi(hi) {
            this.highlightArrows(hi);
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
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getOverview('map', '', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.applications = data.map || [];
                this.categories = data.categories || [];
            });
        },
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
                a.downstreams = a.downstreams.filter((d) => !backLinks.has(d.id + '->' + a.id));
            });

            const roots = applications.filter((a) => a.downstreams.length === 0);
            roots.forEach((a) => {
                calcLevel(index, a, 0, backLinks);
            });

            const depth = Math.max(...applications.map((a) => a.level));
            const levels = Array.from({ length: depth + 1 }, () => []);
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
                return { top: el.offsetTop, left: el.offsetLeft, width: el.offsetWidth, height: el.offsetHeight };
            };
            const arrows = [];
            applications.forEach((app) => {
                app.upstreams.forEach((u) => {
                    const a = {
                        src: app.id,
                        dst: u.id,
                        status: u.status,
                        w: u.weight || 0,
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
                    if (a.x1 > a.x2) {
                        a.x1 = s.left;
                        a.x2 = d.left + d.width;
                    }
                    a.d = `M${a.x1},${a.y1} L${a.x2},${a.y2}`;

                    if (u.stats && u.stats.length) {
                        a.stats = { x: (a.x2 + a.x1) / 2 - 20, y: (a.y2 + a.y1) / 2 - (u.stats.length * 12) / 2, items: u.stats };
                    }
                });
            });
            this.arrows = arrows;
        },
        highlightArrows(hiApp) {
            this.arrows.forEach((a) => {
                a.hi = hiApp && (a.src === hiApp || a.dst === hiApp);
                a.dd = '';
            });
            if (!hiApp) {
                return;
            }
            const hiArrows = this.arrows.filter((a) => a.hi);
            const maxW = Math.max(...hiArrows.map((a) => a.w));
            if (!maxW) {
                return;
            }
            hiArrows.forEach((a) => {
                if (!a.w) {
                    return;
                }
                const w = (3 * a.w) / maxW;
                const r = w / 2 + ((a.y2 - a.y1) ** 2 + (a.x2 - a.x1) ** 2) / (8 * w);
                a.dd = `M${a.x1},${a.y1} A${r},${r} 0,0,0 ${a.x2},${a.y2} A${r},${r} 0,0,0 ${a.x1},${a.y1}`;
            });
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
    border: 1px solid #bdbdbd;
    border-radius: 3px;
    white-space: nowrap;
    padding: 4px 8px;
    background-color: var(--background-color);
    display: inline-flex;
    flex-direction: column;
    line-height: 1.1;
    text-align: left;
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
.arrow.unknown {
    fill: var(--status-unknown);
    stroke: var(--status-unknown);
    stroke-dasharray: 4;
}
.arrow.ok {
    fill: var(--status-ok);
    stroke: var(--status-ok);
}
.arrow.warning {
    fill: var(--status-warning);
    stroke: var(--status-warning);
    stroke-dasharray: 6;
    stroke-width: 1.5;
}
.arrow.critical {
    fill: var(--status-critical);
    stroke: var(--status-critical);
    stroke-dasharray: 6;
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
    padding: 2px;
    border-radius: 2px;
    text-align: right;
}
</style>
