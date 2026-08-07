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
import { applyTheme, normalizeTheme } from '@/utils/theme';

export default {
    props: {
        compact: {
            type: Boolean,
            default: false,
        },
    },

    data() {
        return {
            theme: normalizeTheme(this.$storage.local('theme') || 'dark'),
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
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
            if (normalizeTheme(this.$storage.local('theme') || 'dark') === 'auto') {
                this.setTheme();
            }
        });
    },

    methods: {
        setTheme(theme) {
            this.theme = applyTheme(this.$vuetify, theme);
        },
    },
};
</script>
