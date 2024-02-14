<template>
    <v-menu dark offset-y tile left attach=".v-app-bar">
        <template #activator="{ on }">
            <v-btn v-on="on" plain outlined height="40" class="px-2">
                <v-icon>{{ themes[theme] }}</v-icon>
            </v-btn>
        </template>
        <v-list dense>
            <v-list-item-group v-model="theme">
                <v-list-item v-for="(icon, name) in themes" @click="setTheme(name)" :value="name">
                    <v-icon small class="mr-1">{{ icon }}</v-icon>
                    {{ name }}
                </v-list-item>
            </v-list-item-group>
        </v-list>
    </v-menu>
</template>

<script>
export default {
    data() {
        return {
            theme: this.$storage.local('theme') || 'auto',
        };
    },

    computed: {
        themes() {
            return {
                light: 'mdi-weather-sunny',
                dark: 'mdi-weather-night',
                auto: 'mdi-theme-light-dark',
            };
        },
    },

    mounted() {
        this.setTheme();
    },

    methods: {
        setTheme(theme) {
            const matchMedia = window.matchMedia('(prefers-color-scheme: dark)');
            if (theme) {
                this.theme = theme;
                this.$storage.local('theme', this.theme);
            } else {
                matchMedia.addEventListener('change', (e) => {
                    const theme = this.$storage.local('theme') || 'auto';
                    if (theme === 'auto') {
                        this.$vuetify.theme.dark = e.matches;
                    }
                });
            }
            this.theme = this.$storage.local('theme') || 'auto';
            if (this.theme === 'auto') {
                this.$vuetify.theme.dark = matchMedia.matches;
            } else {
                this.$vuetify.theme.dark = this.theme === 'dark';
            }
        },
    },
};
</script>
