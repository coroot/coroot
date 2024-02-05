<template>
    <v-tooltip bottom>
        <template #activator="{ on }">
            <span v-on="on">
                <Led :status="app.status" />
                <span>{{ $utils.appId(app.id).name }}</span>
            </span>
        </template>
        <div v-for="i in app.indicators">
            <Led :status="i.status" />
            <span>{{ i.message }}</span>
        </div>
    </v-tooltip>
</template>

<script>
import Led from './Led';

export default {
    props: {
        app: Object,
    },
    components: { Led },
};
</script>

<style scoped></style>
