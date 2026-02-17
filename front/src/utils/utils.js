const emptyJson = JSON.stringify({});

export default class Utils {
    router = null;

    constructor(router) {
        this.router = router;
    }

    stateToUri(s) {
        const j = JSON.stringify(s);
        const hash = j === emptyJson ? undefined : '#' + encodeURIComponent(j);
        this.router.replace({ hash }).catch((err) => err);
    }

    stateFromUri() {
        const j = decodeURIComponent(this.router.currentRoute.hash.substring(1));
        if (!j) {
            return {};
        }
        try {
            return JSON.parse(j);
        } catch {
            this.router.replace({ hash: undefined });
            return {};
        }
    }

    appId(id) {
        const parts = id.split(':');
        return {
            cluster: parts[0],
            ns: parts[1] !== '_' ? parts[1] : '',
            kind: parts[2],
            name: parts[4] ? parts[3] + ':' + parts[4] : parts[3],
        };
    }

    nodeId(id) {
        const parts = id.split(':');
        return {
            cluster: parts[0],
            name: parts[1],
        };
    }

    contextQuery() {
        const r = this.router.currentRoute;
        if (!r) {
            return {};
        }
        const { from, to, incident, alert } = r.query || {};
        return { from, to, incident, alert };
    }
}
