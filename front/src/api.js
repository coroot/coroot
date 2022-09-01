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

    contextWatch(component, cb) {
        component.$watch('$route', (newVal, oldVal) => {
            if (
                newVal.params.projectId !== oldVal.params.projectId ||
                newVal.query.from !== oldVal.query.from ||
                newVal.query.to !== oldVal.query.to
            ) {
                cb();
            }
        })
    }

    request(req, cb) {
        this.axios(req).then((response) => {
            try {
                cb(response.data, '');
            } catch (e) {
                console.error(e);
            }
        }).catch((error) => {
            const err = error.response && error.response.data && error.response.data.trim() || defaultErrorMessage;
            cb(null, err);
        })
    }

    get(url, cb) {
        const q = this.router.currentRoute.query;
        const params = {from: q.from, to: q.to};
        this.request({method: 'get', url, params}, cb);
    }

    post(url, data, cb) {
        this.request({method: 'post', url, data}, cb);
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
