import Vue from 'vue';
import Vuetify from 'vuetify/lib';

Vue.use(Vuetify);

// Stackblaze dashboard tokens (dashboard/src/styles/tailwind.css)
// Light → VS Code Visual Studio Light | Dark → VS Code Dark Modern
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
        options: {
            customProperties: true,
        },
        themes: {
            light: {
                primary: primaryLight,
                secondary: '#f3f3f3',
                accent: primaryLight,
                error: '#c72e0f',
                warning: '#b89500',
                info: '#0284c7',
                success: '#098658',
                background: '#ffffff',
            },
            dark: {
                primary: primaryDark,
                secondary: '#2b2b2b',
                accent: primaryDark,
                error: '#f85149',
                warning: '#9e6a03',
                info: '#0ea5e9',
                success: '#2ea043',
                background: '#1f1f1f',
            },
        },
    },
});
