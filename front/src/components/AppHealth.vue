<template>
    <v-tooltip bottom>
        <template #activator="{ on }">
            <span v-on="on">
                <Led :status="app.status" />{{$utils.appId(app.id).name}}
            </span>
        </template>
        <div v-for="i in app.indicators">
            <Led :status="i.status" />{{i.message}}
        </div>
    </v-tooltip>
</template>

<script>
import Led from "./Led";

export default {
    props: {
        app: Object,
    },
    components: {Led},
}
</script>

<style scoped>

</style>