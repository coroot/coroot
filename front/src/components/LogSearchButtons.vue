<template>
    <div class="d-flex flex-nowrap buttons">
        <v-btn @click="search" color="primary" height="40" tile depressed> Show logs </v-btn>
        <v-menu offset-y left>
            <template v-slot:activator="{ on }">
                <v-btn v-on="on" color="primary" height="40" tile depressed min-width="unset" class="px-2">
                    <v-icon small>mdi-refresh</v-icon>
                    <span v-if="interval" class="mx-1">{{ interval }}s</span>
                    <v-icon small>mdi-chevron-down</v-icon>
                </v-btn>
            </template>
            <v-list dense class="py-0">
                <v-subheader>AUTO-REFRESH</v-subheader>
                <v-list-item v-for="i in intervals" :key="i" @click="refresh(i)" :class="{ 'v-list-item--active': interval === i }">
                    <v-list-item-content>
                        <v-list-item-title>{{ i ? i + 's' : 'Off' }}</v-list-item-title>
                    </v-list-item-content>
                </v-list-item>
            </v-list>
        </v-menu>
    </div>
</template>

<script>
export default {
    props: {
        interval: Number,
    },

    computed: {
        intervals() {
            return [0, 5, 10, 30];
        },
    },

    methods: {
        search() {
            this.$emit('search');
        },
        refresh(interval) {
            this.$emit('refresh', interval);
        },
    },
};
</script>

<style scoped>
.buttons {
    flex-shrink: 0;
    align-self: flex-start;
}
.buttons .v-btn:first-child {
    border-radius: 6px 0 0 6px !important;
}
.buttons .v-btn:last-child {
    border-radius: 0 6px 6px 0 !important;
}
.buttons .v-btn:not(:last-child) {
    border-right: thin solid rgba(255, 255, 255, 0.25) !important;
}
</style>
