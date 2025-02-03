<template>
    <v-menu offset-y>
        <template v-slot:activator="{ attrs, on }">
            <v-btn icon x-small class="ml-1" v-bind="attrs" v-on="on">
                <v-icon small>mdi-dots-vertical</v-icon>
            </v-btn>
        </template>

        <v-list dense>
            <template v-if="app.category && app.category !== 'application'">
                <v-list-item
                    link
                    :to="{
                        name: 'project_settings',
                        params: { tab: 'applications' },
                        hash: '#categories',
                        query: {
                            category: app.category,
                        },
                    }"
                >
                    <v-list-item-title>Edit the "{{ app.category }}" category</v-list-item-title>
                </v-list-item>
            </template>

            <v-list-item class="grey--text">Move the app to a category</v-list-item>
            <v-list-item
                link
                :to="{
                    name: 'project_settings',
                    params: { tab: 'applications' },
                    hash: '#categories',
                    query: {
                        app_pattern: appPattern(),
                    },
                }"
            >
                <v-list-item-title> <v-icon small class="mr-2">mdi-plus</v-icon>a new category</v-list-item-title>
            </v-list-item>
            <template v-if="categories">
                <v-list-item
                    v-for="c in categories"
                    link
                    :to="{
                        name: 'project_settings',
                        params: { tab: 'applications' },
                        hash: '#categories',
                        query: {
                            category: c,
                            app_pattern: appPattern(),
                        },
                    }"
                >
                    <v-icon small class="mr-2">mdi-arrow-right-thin</v-icon>
                    <v-list-item-title>{{ c }}</v-list-item-title>
                </v-list-item>
            </template>
            <v-list-item
                v-if="app.custom"
                link
                :to="{
                    name: 'project_settings',
                    params: { tab: 'applications' },
                    hash: '#custom-applications',
                    query: { custom_app: $utils.appId(app.id).name },
                }"
            >
                <v-list-item-title>Edit custom application</v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script>
export default {
    props: {
        app: Object,
        categories: [],
    },
    methods: {
        appPattern() {
            const id = this.$utils.appId(this.app.id);
            const ns = id.ns ? id.ns : '_';
            return ns + '/' + id.name;
        },
    },
};
</script>

<style scoped></style>
