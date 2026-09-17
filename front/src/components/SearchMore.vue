<template>
    <div class="pa-4 text-center grey--text">
        No applications found in this view.
        <v-btn text small color="primary" @click="$events.openSearch(query)">
            <v-icon small left>mdi-magnify</v-icon>
            Search all apps and nodes
        </v-btn>
    </div>
</template>

<script>
export default {
    props: {
        query: String,
    },
};
</script>
