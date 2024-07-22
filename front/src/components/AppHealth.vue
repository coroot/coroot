<template>
    <v-tooltip v-if="app.indicators" bottom>
        <template #activator="{ on }">
            <span v-on="on">
                <Led :status="app.status" />
                <span>{{ $utils.appId(app.id).name }}</span>
            </span>
        </template>
        <v-card class="pa-2">
            <div v-for="i in app.indicators">
                <Led :status="i.status" />
                <span>{{ i.message }}</span>
            </div>
        </v-card>
    </v-tooltip>
    <span v-else>
        <Led :status="app.status" />
        <span>{{ $utils.appId(app.id).name }}</span>
    </span>
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
