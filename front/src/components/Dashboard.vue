<template>
    <div class="d-flex flex-wrap">
        <Widget v-for="w in widgets" :w="w" class="my-5" :style="{width: $vuetify.breakpoint.mdAndUp ? (w.width || '50%') : '100%'}" />
    </div>
</template>

<script>
import Widget from "@/components/Widget";

export default {
    props: {
        widgets: Array,
    },

    components: {Widget},
}
</script>

<style scoped>

</style>
