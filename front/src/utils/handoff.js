import * as storage from '@/utils/storage';

// Signals that travel with the Kubero handoff exactly like `theme` does:
//   return_url — absolute URL of the dashboard page that opened Coroot.
//   workspace  — workspace/tenant display name shown in the sidebar header.
// Transport mirrors theme: ?return_url=/?workspace= query params on the entry
// URL, and coroot_return_url/coroot_workspace cookies (set by the Go handoff
// handler) that survive the base-path redirect.
const RETURN_URL_COOKIE = 'coroot_return_url';
const WORKSPACE_COOKIE = 'coroot_workspace';

function cookieValue(name) {
    const match = document.cookie.match(new RegExp('(?:^|; )' + name + '=([^;]*)(?:;|$)'));
    if (!match) {
        return null;
    }
    try {
        return decodeURIComponent(match[1]);
    } catch {
        return match[1];
    }
}

function clearCookie(name) {
    // Value is no longer needed once persisted to localStorage.
    document.cookie = `${name}=; Path=/; Max-Age=0; SameSite=Lax`;
}

export function getReturnUrl() {
    return storage.local('return_url') || null;
}

export function getWorkspace() {
    return storage.local('workspace') || null;
}

/**
 * Prefer the Kubero handoff signal (?<param>= / <cookie> cookie), then
 * localStorage. Strips the query param after reading so it doesn't stick in
 * the address bar. Mirrors bootstrapTheme().
 */
function bootstrapSignal(param, cookieName, storageKey) {
    const params = new URLSearchParams(window.location.search);
    const fromUrl = params.get(param);
    const fromCookie = cookieValue(cookieName);
    const chosen = (fromUrl && fromUrl.trim()) || fromCookie;

    if (chosen) {
        storage.local(storageKey, chosen);
    }
    if (fromUrl !== null) {
        params.delete(param);
        const qs = params.toString();
        const next = window.location.pathname + (qs ? `?${qs}` : '') + window.location.hash;
        window.history.replaceState({}, '', next);
    }
    if (fromCookie) {
        clearCookie(cookieName);
    }
    return chosen || null;
}

export function bootstrapHandoff() {
    bootstrapSignal('return_url', RETURN_URL_COOKIE, 'return_url');
    bootstrapSignal('workspace', WORKSPACE_COOKIE, 'workspace');
}
