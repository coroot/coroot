<template>
    <v-list-item-group v-model="theme" :class="{ 'sb-theme-selector--compact': compact }">
        <v-list-item
            v-for="(icon, name) in themes"
            :key="name"
            dense
            :class="{ 'sb-theme-selector__item': compact }"
            @click="setTheme(name)"
            :value="name"
        >
            <v-icon :x-small="compact" :small="!compact" class="mr-1">{{ icon }}</v-icon>
            <span class="text-capitalize">{{ name }}</span>
        </v-list-item>
    </v-list-item-group>
</template>

<script>
export default {
    props: {
        compact: {
            type: Boolean,
            default: false,
        },
    },

    data() {
        return {
            theme: this.$storage.local('theme') || 'dark',
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

    created() {
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
                    const theme = this.$storage.local('theme') || 'dark';
                    if (theme === 'auto') {
                        this.$vuetify.theme.dark = e.matches;
                        this.syncBodyClass();
                    }
                });
            }
            this.theme = this.$storage.local('theme') || 'dark';
            if (this.theme === 'auto') {
                this.$vuetify.theme.dark = matchMedia.matches;
            } else {
                this.$vuetify.theme.dark = this.theme === 'dark';
            }
            this.syncBodyClass();
        },
        syncBodyClass() {
            if (this.$vuetify.theme.dark) {
                document.body.classList.add('theme--dark');
            } else {
                document.body.classList.remove('theme--dark');
            }
        },
    },
};
</script>
