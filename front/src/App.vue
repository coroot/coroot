<template>
<v-app>
    <v-app-bar app flat dark color="#080d1b">
        <v-container class="py-0 fill-height">
            <router-link :to="{name: 'overview'}">
                <img src="/static/logo.svg" height="38" style="vertical-align: middle;">
            </router-link>
            <v-spacer />
            <TimePicker />
        </v-container>
    </v-app-bar>

    <v-main>
        <v-container>
            <router-view />
        </v-container>
    </v-main>
</v-app>
</template>

<script>
import TimePicker from "@/components/TimePicker";

export default {
    components: {TimePicker},
}
</script>

<style>
a {
    text-decoration: none !important;
    color: #0000ee !important;
}
a:visited {
    color: #0000ee !important;
}
.v-btn {
    text-transform: none !important;
    font-weight: normal !important;
    letter-spacing: inherit !important;
    font-size: inherit !important;
}
</style>
