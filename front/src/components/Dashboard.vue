<template>
    <div class="d-flex flex-wrap">
        <Widget
            v-for="(w, i) in widgets"
            :key="name + ':' + i"
            :w="w"
            class="my-5"
            :style="{ width: $vuetify.breakpoint.mdAndUp ? w.width || '50%' : '100%' }"
        />
    </div>
</template>

<script>
import Widget from './Widget';

export default {
    props: {
        name: String,
        widgets: Array,
    },

    components: { Widget },
};
</script>

<style scoped></style>
