import * as storage from '@/utils/storage';

const THEMES = new Set(['light', 'dark', 'auto']);
const THEME_COOKIE = 'coroot_theme';

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

function setVuetifyDark(vuetify, dark) {
    if (!vuetify) return;
    // Plugin export (main.js) has .framework; component this.$vuetify is the framework.
    const theme = vuetify.framework?.theme || vuetify.theme;
    if (theme) theme.dark = !!dark;
}

export function applyTheme(vuetify, theme) {
    const next = normalizeTheme(theme || storage.local('theme') || 'dark');
    storage.local('theme', next);
    const dark = isDark(next);
    setVuetifyDark(vuetify, dark);
    syncBodyClass(dark);
    return next;
}

function themeFromCookie() {
    const match = document.cookie.match(/(?:^|; )coroot_theme=(light|dark|auto)(?:;|$)/);
    return match ? match[1] : null;
}

function clearThemeCookie() {
    // Expire both Path=/ and typical base-path variants; value no longer needed
    // once persisted to localStorage.
    document.cookie = `${THEME_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`;
}

/**
 * Prefer Kubero handoff signals (?theme= / coroot_theme cookie), then localStorage.
 * Strips the query param after reading so it doesn't stick in the address bar.
 */
export function bootstrapTheme(vuetify) {
    const params = new URLSearchParams(window.location.search);
    const fromUrl = params.get('theme');
    const fromCookie = themeFromCookie();
    const chosen = THEMES.has(fromUrl) ? fromUrl : fromCookie;

    if (chosen) {
        applyTheme(vuetify, chosen);
        if (THEMES.has(fromUrl)) {
            params.delete('theme');
            const qs = params.toString();
            const next = window.location.pathname + (qs ? `?${qs}` : '') + window.location.hash;
            window.history.replaceState({}, '', next);
        }
        if (fromCookie) clearThemeCookie();
        return chosen;
    }
    return applyTheme(vuetify, storage.local('theme') || 'dark');
}
