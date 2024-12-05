<template>
    <div>
        <v-row style="margin: -4px">
            <v-col cols="12" sm="6" style="padding: 4px">
                <v-select
                    v-model="instanceInternal"
                    :items="instances"
                    @change="$emit('change:instance', instanceInternal)"
                    label="Select instance"
                    outlined
                    dense
                    hide-details
                    clearable
                    single-line
                    :menu-props="{ offsetY: true }"
                />
            </v-col>
            <v-col cols="12" sm="6" style="padding: 4px">
                <v-text-field v-model="search" dense hide-details clearable prepend-inner-icon="mdi-magnify" label="Search" single-line outlined />
            </v-col>
        </v-row>

        <FlameGraphNode
            v-if="profile.flamegraph"
            :node="profile.flamegraph"
            :parent="profile.flamegraph"
            :root="profile.flamegraph"
            :zoom="zoom"
            @zoom="zoom = true"
            :search="search"
            :diff="diff"
            :unit="unit"
            :limit="limit"
            :actions="actions"
            class="mt-2"
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
        instances: Array,
        instance: String,
        limit: Number,
        actions: Array,
    },

    components: { FlameGraphNode },

    data() {
        return {
            zoom: undefined,
            instanceInternal: this.instance,
            search: '',
        };
    },

    watch: {
        instance(v) {
            this.instanceInternal = v;
        },
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
*:deep(.v-list-item) {
    min-height: 32px !important;
}
</style>
