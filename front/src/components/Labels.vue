<template>
    <span class="labels">
        <span v-for="l in labels" class="label">
            <template v-if="l.Name === 'db'"><v-icon  x-small>mdi-database</v-icon>{{l.Value}}</template>
            <template v-else-if="l.Name === 'queue'"><v-icon x-small>mdi-tray-full</v-icon>{{l.Value}}</template>
            <template v-else-if="l.Name === 'storage'"><v-icon  x-small>mdi-harddisk</v-icon>{{l.Value}}</template>
            <template v-else>{{l.Name}}:{{l.Value}}</template>
        </span>
    </span>
</template>

<script>
    export default {
        props: {
            labels: Array,
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
        color: #9E9E9E;
    }
    .label:not(:last-child):after {
        content: " / ";
    }
</style>
