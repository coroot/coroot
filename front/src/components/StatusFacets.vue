<template>
    <div v-if="visibleFacets.length > 0" class="facets">
        <div v-for="facet in visibleFacets" :key="facet.key" class="facet-row">
            <span class="facet-label">{{ facet.label }}</span>
            <button
                v-for="opt in facet.options"
                :key="opt.value"
                type="button"
                class="chip"
                :class="{ active: selected[facet.key] === opt.value, muted: opt.count === 0 }"
                @click="$emit('toggle', { key: facet.key, value: opt.value })"
            >
                <Led :status="opt.level" />
                <span class="chip-label">{{ opt.value }}</span>
                <span class="chip-count">{{ opt.count }}</span>
            </button>
        </div>
    </div>
</template>

<script>
import Led from './Led';

export default {
    components: { Led },
    props: {
        facets: { type: Array, required: true },
        selected: { type: Object, required: true },
    },
    computed: {
        visibleFacets() {
            return this.facets.filter((f) => f.options.length > 0).map((f) => ({ ...f, options: [...f.options].sort(this.order) }));
        },
    },
    methods: {
        order(a, b) {
            const rank = { ok: 0, warning: 1, critical: 2, unknown: 3 };
            const ra = a.level in rank ? rank[a.level] : 4;
            const rb = b.level in rank ? rank[b.level] : 4;
            return ra !== rb ? ra - rb : a.value.localeCompare(b.value);
        },
    },
};
</script>

<style scoped>
.facets {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 16px;
}
.facet-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
}
.facet-label {
    font-size: 12px;
    color: rgba(128, 128, 128, 0.9);
    min-width: 48px;
}
.chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 1px 8px;
    border: 1px solid rgba(128, 128, 128, 0.4);
    border-radius: 12px;
    font-size: 12px;
    line-height: 18px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    transition:
        background-color 0.15s,
        border-color 0.15s;
}
.chip:hover {
    background-color: rgba(128, 128, 128, 0.12);
}
.chip.active {
    border-color: #1976d2;
    background-color: rgba(25, 118, 210, 0.1);
    color: #1976d2;
}
.chip.muted {
    opacity: 0.45;
}
.chip-count {
    font-weight: 600;
}
</style>
