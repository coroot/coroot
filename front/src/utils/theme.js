import * as storage from '@/utils/storage';

const THEMES = new Set(['light', 'dark', 'auto']);

export function normalizeTheme(value) {
    return THEMES.has(value) ? value : 'dark';
}

export function isDark(theme) {
    const t = normalizeTheme(theme);
    if (t === 'auto') {
        return window.matchMedia('(prefers-color-scheme: dark)').matches;
    }
    return t === 'dark';
}

export function syncBodyClass(dark) {
    document.body.classList.toggle('theme--dark', !!dark);
}

/**
 * Apply Coroot theme to Vuetify + body, and persist.
 * @param {import('vuetify').default} vuetify
 * @param {string} [theme]
 */
function setVuetifyDark(vuetify, dark) {
    if (!vuetify) return;
    // Plugin export (main.js) has .framework; component this.$vuetify is the framework.
    const theme = vuetify.framework?.theme || vuetify.theme;
    if (theme) theme.dark = dark;
}

export function applyTheme(vuetify, theme) {
    const next = normalizeTheme(theme || storage.local('theme') || 'dark');
    storage.local('theme', next);
    const dark = isDark(next);
    setVuetifyDark(vuetify, dark);
    syncBodyClass(dark);
    return next;
}

/**
 * Prefer ?theme= from the URL (Kubero handoff), then localStorage.
 * Strips the query param after reading so it doesn't stick in history.
 */
export function bootstrapTheme(vuetify) {
    const params = new URLSearchParams(window.location.search);
    const fromUrl = params.get('theme');
    if (THEMES.has(fromUrl)) {
        applyTheme(vuetify, fromUrl);
        params.delete('theme');
        const qs = params.toString();
        const next = window.location.pathname + (qs ? `?${qs}` : '') + window.location.hash;
        window.history.replaceState({}, '', next);
        return fromUrl;
    }
    return applyTheme(vuetify, storage.local('theme') || 'dark');
}
