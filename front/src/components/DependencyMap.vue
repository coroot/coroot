<template>
    <div v-on-resize="calc" class="wrapper">
        <div class="map">
            <div v-for="az in azs" class="az">
                <div class="az-title" title="availability zone">{{ az.name }}</div>
                <div class="az-body">
                    <div v-for="n in az.nodes" class="node" :class="n.status" title="node">
                        <div class="node-title">{{ n.name }}</div>
                        <div class="node-body">
                            <div class="src">
                                <div
                                    v-for="i in n.src_instances"
                                    :data-id="'src-' + i.id"
                                    class="instance"
                                    :class="{ obsolete: i.obsolete }"
                                    :title="'instance' + (i.obsolete ? ' (obsolete)' : '')"
                                >
                                    {{ i.name }}
                                </div>
                            </div>
                            <div class="dst">
                                <div
                                    v-for="i in n.dst_instances"
                                    :data-id="'dst-' + i.id"
                                    class="instance"
                                    :class="{ obsolete: i.obsolete }"
                                    :title="'instance' + (i.obsolete ? ' (obsolete)' : '')"
                                >
                                    {{ i.name }}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <svg>
            <line v-for="l in lines" :x1="l.x1" :y1="l.y1" :x2="l.x2" :y2="l.y2" :class="l.status" />
        </svg>
    </div>
</template>

<script>
export default {
    props: {
        nodes: Array,
        links: Array,
    },

    data() {
        return {
            azs: [],
            lines: [],
        };
    },

    mounted() {
        this.$nextTick(this.calc);
    },

    watch: {
        links() {
            this.$nextTick(this.calc);
        },
    },

    methods: {
        calc() {
            const nodes = this.nodes || [];
            let azs = new Map();
            nodes.forEach((n) => {
                const k = `${n.provider}:${n.pegion}:${n.az}`;
                if (!azs.has(k)) {
                    azs.set(k, { name: (n.provider ? n.provider + ':' : '') + n.az, nodes: new Map() });
                }
                azs.get(k).nodes.set(n.name, n);
            });
            azs = Array.from(azs.values());
            const byName = (a, b) => a.name.localeCompare(b.name);
            azs.sort(byName);
            azs.forEach((az) => {
                az.nodes = Array.from(az.nodes.values());
                az.nodes.sort(byName);
            });
            this.azs = azs;

            const rects = new Map();
            Array.from(this.$el.getElementsByClassName('instance')).forEach((el) => {
                rects.set(el.dataset.id, { top: el.offsetTop, left: el.offsetLeft, width: el.offsetWidth, height: el.offsetHeight });
            });
            if (rects.size === 0) {
                return;
            }
            const links = this.links || [];
            this.lines = links.map((l) => {
                const s = rects.get('src-' + l.src_instance);
                const d = rects.get('dst-' + l.dst_instance);
                const x1 = s.left + s.width;
                const y1 = s.top + s.height / 2;
                const x2 = d.left;
                const y2 = d.top + d.height / 2;
                const status = l.status;
                return { x1, y1, x2, y2, status };
            });
        },
    },
};
</script>

<style scoped>
.wrapper {
    position: relative;
}
.az {
    padding: 4px 8px;
    border: 1px dashed #bdbdbd;
    margin: 16px 0;
    border-radius: 8px;
}
.az-title {
    font-size: 0.9rem;
    color: #9e9e9e;
    padding-bottom: 4px;
    text-transform: lowercase;
}
.node {
    padding: 4px;
    border: 1px solid #bdbdbd;
    border-radius: 3px;
    white-space: nowrap;
    margin: 8px 8px;
}
.node-title {
    padding: 4px;
    max-width: 90%;
    overflow: hidden;
    text-overflow: ellipsis;
}
.node-body {
    display: flex;
    justify-content: space-between;
}
.node-body * {
    align-self: center;
    min-width: 40%;
    max-width: 40%;
}
.node-body .dst .instance {
    margin-left: auto;
}
.node.warning {
    background-color: var(--background-color-hi) !important;
    border-color: var(--background-color-hi) !important;
}
.instance {
    padding: 4px;
    border: 1px solid #bdbdbd;
    border-radius: 3px;
    font-size: 0.75rem;
    margin: 8px;
    max-width: 80%;
    overflow: hidden;
    text-overflow: ellipsis;
}
.instance.obsolete {
    color: var(--text-color-dimmed);
}
svg {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none; /* to allow interactions with html below */
}
line.ok {
    stroke: var(--status-ok);
    stroke-width: 1;
}
line.warning {
    stroke: var(--status-warning);
    stroke-width: 2;
    stroke-dasharray: 6;
}
line.critical {
    stroke: var(--status-critical);
    stroke-width: 2;
    stroke-dasharray: 6;
}
line.unknown {
    stroke: var(--status-unknown);
    stroke-width: 1;
    stroke-dasharray: 4;
}
</style>
