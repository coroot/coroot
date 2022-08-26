import axios from "axios";

export default class Api {
    axios = axios.create({
        baseURL: '/api/',
        timeout: 30000,
    });
    router = null;

    constructor(router) {
        this.router = router;
    }

    appId(id) {
        const parts = id.split(':');
        return {
            ns: parts[0] !== '_' ? parts[0] : '',
            kind: parts[1],
            name: parts[3] ? ':'+parts[3] : parts[2],
        }
    }

    timeContextWatch(component, cb) {
        component.$watch('$route.query', (newVal, oldVal) => {
            if (newVal.from !== oldVal.from || newVal.to !== oldVal.to) {
                cb();
            }
        })
    }

    get(url, cb) {
        const q = this.router.currentRoute.query;
        const params = {from: q.from, to: q.to};
        this.axios.get(url, {params}).then((response) => {
            cb(response.data, '');
        }).catch((error) => {
            cb(null, error.response.data && error.response.data.trim() || 'Something went wrong, please try again later.');
        })
    }

    getOverview(cb) {
        this.get(`overview`, cb);
    }

    getApplication(id, cb) {
        this.get(`app/${id}`, cb);
    }

    getNode(name, cb) {
        this.get(`node/${name}`, cb);
    }

    search(cb) {
        this.get(`search`, cb);
    }
}
