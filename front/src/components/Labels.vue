<template>
    <span class="labels">
        <span v-for="k in keys" class="label">
            <template v-if="k === 'db'"><v-icon x-small>mdi-database</v-icon>{{ labels[k] }}</template>
            <template v-else-if="k === 'queue'"><v-icon x-small>mdi-tray-full</v-icon>{{ labels[k] }}</template>
            <template v-else>{{ k }}:{{ labels[k] }}</template>
        </span>
    </span>
</template>

<script>
export default {
    props: {
        labels: Object,
    },

    computed: {
        keys() {
            if (!this.labels) {
                return [];
            }
            const res = Object.keys(this.labels);
            res.sort((a, b) => a.localeCompare(b));
            return res;
        },
    },
};
</script>

<style scoped>
.labels {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
}
.label {
    font-size: 0.75rem;
    color: #9e9e9e;
}
.label:not(:last-child):after {
    content: ' / ';
}
</style>
