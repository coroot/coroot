import axios from "axios";

const defaultErrorMessage = 'Something went wrong, please try again later.';

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
            const err = error.response.data && error.response.data.trim() || defaultErrorMessage;
            cb(null, err);
        })
    }

    post(url, data, cb) {
        this.axios.post(url, data).then((response) => {
            cb(response.data, '');
        }).catch((error) => {
            const err = error.response.data && error.response.data.trim() || defaultErrorMessage;
            cb(null, err);
        })
    }

    getProjects(cb) {
        this.get(`projects`, cb);
    }

    getProject(projectId, cb) {
        this.get(`project/${projectId || ''}`, cb);
    }

    saveProject(projectId, form, cb) {
        this.post(`project/${projectId || ''}`, form, cb);
    }

    projectPath(subPath) {
        const projectId = this.router.currentRoute.params.projectId;
        return `project/${projectId}/${subPath}`;
    }

    getOverview(cb) {
        this.get(this.projectPath(`overview`), cb);
    }

    getApplication(appId, cb) {
        this.get(this.projectPath(`app/${appId}`), cb);
    }

    getNode(nodeName, cb) {
        this.get(this.projectPath(`node/${nodeName}`), cb);
    }

    search(cb) {
        this.get(this.projectPath(`search`), cb);
    }
}
