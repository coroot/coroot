<template>
    <div class="incidents" :style="style">
        <div v-for="i in items" class="item" :style="i.style" />
    </div>
</template>

<script>
export default {
    props: {
        ctx: Object,
        bbox: Object,
        incidents: Array,
    },

    computed: {
        style() {
            if (!this.incidents.length || !this.bbox) {
                return { display: 'none' };
            }
            const height = 3;
            const b = this.bbox;
            return {
                display: 'block',
                height: height + 'px',
                top: b.top + b.height + height + 'px',
                left: b.left + 'px',
                width: b.width + 'px',
            };
        },
        items() {
            if (!this.incidents.length || !this.bbox) {
                return [];
            }
            const ctx = this.ctx;
            const b = this.bbox;
            const norm = (x) => (x - ctx.from) / (ctx.to - ctx.from);
            return this.incidents.map((i) => {
                const x1 = Math.max(0, b.width * norm(i.x1 - ctx.step / 2));
                const x2 = Math.min(b.width, b.width * norm(i.x2 + ctx.step / 2));
                return {
                    style: {
                        left: x1 + 'px',
                        width: x2 - x1 + 'px',
                    },
                };
            });
        },
    },
};
</script>

<style scoped>
.incidents {
    position: absolute;
    z-index: 1;
    background-color: hsl(141, 50%, 70%);
}
.item {
    position: absolute;
    height: 100%;
    background-color: hsl(4, 90%, 60%);
}
</style>
