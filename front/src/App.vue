<template>
<v-app>
    <v-app-bar app flat dark color="#080d1b">
        <v-container>
            <router-link :to="{name: 'overview'}">
                <img src="/static/logo.svg" height="38" style="vertical-align: middle;">
            </router-link>
        </v-container>
    </v-app-bar>

    <v-main>
        <v-container>
            <router-view />
        </v-container>
    </v-main>
</v-app>
</template>

<style>
a {
    text-decoration: none !important;
    color: #0000ee !important;
}
a:visited {
    color: #0000ee !important;
}
</style>
