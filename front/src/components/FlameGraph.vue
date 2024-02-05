<template>
    <div v-if="profile.flamegraph">
        <v-text-field
            v-model="search"
            dense
            hide-details
            clearable
            prepend-inner-icon="mdi-magnify"
            label="Search"
            single-line
            outlined
            class="search"
        />

        <FlameGraphNode
            :node="profile.flamegraph"
            :parent="profile.flamegraph"
            :root="profile.flamegraph"
            :zoom="zoom"
            @zoom="zoom = true"
            :search="search"
            :diff="diff"
            :unit="unit"
        />
    </div>
</template>

<script>
import FlameGraphNode from './FlameGraphNode.vue';

function maxDiff(root, node) {
    const baseDiff = (node.total - node.comp) / (root.total - root.comp);
    const compDiff = node.comp / root.comp;
    const diff = Math.abs(compDiff - baseDiff);
    return Math.max(diff, ...(node.children || []).map((ch) => maxDiff(root, ch)));
}

export default {
    props: {
        profile: Object,
    },

    components: { FlameGraphNode },

    data() {
        return {
            zoom: undefined,
            search: '',
        };
    },

    computed: {
        unit() {
            return this.profile.type.split(':')[2] || '';
        },
        diff() {
            if (!this.profile.diff) {
                return 0;
            }
            return Math.max(5, maxDiff(this.profile.flamegraph, this.profile.flamegraph) * 100);
        },
    },
};
</script>

<style scoped>
.search {
    margin-bottom: 12px;
}
</style>
