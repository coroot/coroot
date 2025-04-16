import Vue from 'vue';
import Vuetify from 'vuetify/lib';
import colors from 'vuetify/lib/util/colors';

Vue.use(Vuetify);

export default new Vuetify({
    icons: {
        iconfont: 'mdi',
    },
    theme: {
        themes: {
            light: {
                secondary: colors.blue.lighten1,
            },
            dark: {
                secondary: colors.blue.lighten1,
            },
        },
    },
});
