const cookieMaxAge = `; max-age=${365 * 24 * 60 * 60}`;
function deviceSize() {
    const w = document.documentElement.clientWidth;
    if (w < 600) return 'xs';
    if (w < 960) return 'sm';
    if (w < 1280) return 'md';
    if (w < 1920) return 'lg';
    return 'xl';
}
function getId(cookies, name) {
    let id = '';
    const cookie = cookies.find(s => s.trim().startsWith(name));
    if (cookie) {
        id = cookie.split('=')[1];
        if (id) {
            return id;
        }
    }
    let storage = localStorage;
    let maxAge = cookieMaxAge;
    if (name === 'st-session-id') {
        storage = sessionStorage;
        maxAge = '';
    }
    id = storage.getItem(name);
    if (!id) {
        id = crypto.randomUUID();
        storage.setItem(name, id);
    }
    document.cookie = `${name}=${id}; path=/; domain=.coroot.com${maxAge}`;
    return id;
}
function send(eventType) {
    const loc = window.location;
    const cookies = document.cookie.split(';');
    const payload = {
        timestamp: Date.now(),
        deviceId: getId(cookies,'st-device-id'),
        sessionId: getId(cookies,'st-session-id'),
        deviceSize: deviceSize(),
        path: loc.pathname + loc.search + loc.hash,
        referrer: document.referrer,
        eventType: eventType,
    }
    navigator.sendBeacon('https://coroot.com/st', JSON.stringify(payload));
}
addEventListener("pageshow", (event) => {
    send('route-open');
})
addEventListener("visibilitychange", (event) => {
    send('document-visibility-' + document.visibilityState);
});
addEventListener("beforeunload", (event) => {
    send('window-unload');
});
const history = window.history;
if (history.pushState) {
    const pushState = history.pushState;
    history.pushState = function() {
        pushState.apply(this, arguments);
        send('route-open');
    }
    window.addEventListener('popstate', (event) => {
        send('route-open');
    })
}
if (history.replaceState) {
    const replaceState = history.replaceState;
    history.replaceState = function() {
        replaceState.apply(this, arguments);
        send('route-open');
    }
}