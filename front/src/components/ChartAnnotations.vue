<template>
    <div class="annotations">
        <div v-for="(i, idx) in items" :key="idx" class="annotation" :style="i.style">
            <v-menu open-on-hover bottom offset-y :close-on-content-click="false" :nudge-bottom="2" max-height="360">
                <template #activator="{ on }">
                    <span
                        v-on="on"
                        :style="i.total === 1 && i.items[0].link ? 'cursor: pointer' : ''"
                        @click="i.total === 1 && navigate(i.items[0].link)"
                    >
                        <v-badge v-if="i.total > 1" :content="i.total" color="grey darken-1" overlap>
                            <v-icon small>{{ i.icon }}</v-icon>
                        </v-badge>
                        <v-icon v-else small>{{ i.icon }}</v-icon>
                    </span>
                </template>
                <v-card class="pa-1">
                    <div
                        v-for="(it, j) in i.items"
                        :key="j"
                        class="annotation-item d-flex align-center pa-1"
                        :style="it.link ? 'cursor: pointer' : ''"
                        @click="navigate(it.link)"
                    >
                        <v-icon small class="mr-2">{{ it.icon }}</v-icon>
                        <span class="time mr-2">{{ $format.date(it.time, '{HH}:{mm}:{ss}') }}</span>
                        <span v-html="it.msg" />
                        <span v-if="it.count > 1" class="count ml-2">×{{ it.count }}</span>
                    </div>
                </v-card>
            </v-menu>

            <div class="line"></div>
        </div>
    </div>
</template>

<script>
const CLUSTER_PX = 16;

export default {
    props: {
        ctx: Object,
        bbox: Object,
        annotations: Array,
    },

    computed: {
        items() {
            if (!this.annotations.length || !this.bbox) {
                return [];
            }
            const ctx = this.ctx;
            const b = this.bbox;
            const norm = (x) => (x - ctx.from) / (ctx.to - ctx.from);
            const px = (a) => b.left + b.width * norm(a.x);

            const positioned = this.annotations.map((a) => ({ a, x: px(a) })).sort((p, q) => p.x - q.x);

            const clusters = [];
            for (const { a, x } of positioned) {
                const last = clusters[clusters.length - 1];
                if (last && x - last.lastX <= CLUSTER_PX) {
                    last.members.push(a);
                    last.lastX = x;
                } else {
                    clusters.push({ x, lastX: x, members: [a] });
                }
            }

            return clusters.map((c) => {
                const byMsg = new Map();
                const items = [];
                for (const a of c.members) {
                    const existing = byMsg.get(a.msg);
                    if (existing) {
                        existing.count++;
                        continue;
                    }
                    const it = { msg: a.msg, icon: a.icon || 'mdi-alert-circle-outline', time: a.x, link: null, count: 1 };
                    if (a.link) {
                        it.link = { ...a.link, query: { ...this.$route.query, ...a.link.query } };
                    }
                    byMsg.set(a.msg, it);
                    items.push(it);
                }
                return {
                    items,
                    total: c.members.length,
                    icon: items[0].icon,
                    style: {
                        left: c.x + 'px',
                        height: b.top + b.height + 'px',
                    },
                };
            });
        },
    },

    methods: {
        navigate(link) {
            if (link) {
                this.$router.push(link).catch((err) => err);
            }
        },
    },
};
</script>

<style scoped>
.annotation {
    z-index: 1;
    position: absolute;
    transition: none;
    display: flex;
    flex-direction: column;
    width: 0;
}
.line {
    flex-grow: 1;
    border-left: 0.08rem dashed var(--text-color);
    margin-left: -0.04rem;
    pointer-events: none;
}
.annotation-item {
    white-space: nowrap;
    border-radius: 4px;
}
.annotation-item:hover {
    background-color: var(--background-color-hi);
}
.time {
    color: var(--text-color-dimmed, #9e9e9e);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
}
.count {
    color: var(--text-color-dimmed, #9e9e9e);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
}
</style>
