<template>
    <div v-if="badges && badges.length" class="badges">
        <div v-for="b in badges" class="badge" :class="colors[b.status]">{{b.value}}</div>
    </div>
</template>

<script>
import {statuses} from "../utils/colors";

export default {
    props: {
        badges: Array,
    },
    computed: {
        colors() {
            return statuses;
        },
    },
}
</script>

<style scoped>
.badges {
    display: flex;
    font-size: 12px;
    letter-spacing: normal;
    border-radius: 2px;
}
.badge {
    padding: 1px 4px;
    color: white;
    font-weight: normal;
}
.badge:first-child {
    border-top-left-radius: inherit;
    border-bottom-left-radius: inherit
}
.badge:last-child {
    border-top-right-radius: inherit;
    border-bottom-right-radius: inherit
}
</style>