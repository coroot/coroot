<template>
    <div class="annotations">
        <div v-for="i in items" class="annotation" :style="i.style">
            <v-tooltip bottom>
                <template #activator="{ on }">
                    <v-icon v-on="on" small :style="i.link ? 'cursor: pointer' : ''" @click="navigate(i.link)">{{ i.icon }}</v-icon>
                </template>
                <v-card v-html="i.msg" class="pa-2 text-center" />
            </v-tooltip>
            <div class="line"></div>
        </div>
    </div>
</template>

<script>
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
            return this.annotations.map((a) => {
                const item = {
                    msg: a.msg,
                    icon: a.icon || 'mdi-alert-circle-outline',
                    style: {
                        left: b.left + b.width * norm(a.x) + 'px',
                        height: b.top + b.height + 'px',
                    },
                };
                if (a.link) {
                    item.link = { ...a.link, query: { ...this.$route.query, ...a.link.query } };
                }
                return item;
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
</style>
