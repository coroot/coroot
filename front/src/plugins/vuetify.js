import Vue from 'vue';
import Vuetify from 'vuetify/lib';

Vue.use(Vuetify);

// Stackblaze dashboard tokens (dashboard/src/styles/tailwind.css)
const primaryLight = '#5d54a4';
const primaryDark = '#6b26d9';

function initialDark() {
    try {
        const params = new URLSearchParams(window.location.search);
        const fromUrl = params.get('theme');
        if (fromUrl === 'light') return false;
        if (fromUrl === 'dark') return true;
        if (fromUrl === 'auto') {
            return window.matchMedia('(prefers-color-scheme: dark)').matches;
        }
        const cookie = document.cookie.match(/(?:^|; )coroot_theme=(light|dark|auto)(?:;|$)/);
        if (cookie) {
            if (cookie[1] === 'light') return false;
            if (cookie[1] === 'dark') return true;
            return window.matchMedia('(prefers-color-scheme: dark)').matches;
        }
        const data = JSON.parse(localStorage.getItem('coroot') || '{}');
        const t = data.theme || 'dark';
        if (t === 'light') return false;
        if (t === 'auto') {
            return window.matchMedia('(prefers-color-scheme: dark)').matches;
        }
        return true;
    } catch {
        return true;
    }
}

export default new Vuetify({
    icons: {
        iconfont: 'mdi',
    },
    theme: {
        dark: initialDark(),
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
