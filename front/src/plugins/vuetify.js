import Vue from 'vue';
import Vuetify from 'vuetify/lib';

Vue.use(Vuetify);

// Stackblaze dashboard tokens (dashboard/src/styles/tailwind.css)
const primaryLight = '#5d54a4';
const primaryDark = '#6b26d9';

export default new Vuetify({
    icons: {
        iconfont: 'mdi',
    },
    theme: {
        themes: {
            light: {
                primary: primaryLight,
                secondary: primaryLight,
                accent: primaryLight,
            },
            dark: {
                primary: primaryDark,
                secondary: primaryDark,
                accent: primaryDark,
            },
        },
    },
});
