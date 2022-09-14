import axios from "axios";
import * as storage from "@/utils/storage";
import {v4} from 'uuid';

const defaultErrorMessage = 'Something went wrong, please try again later.';

export default class Api {
    axios = null;
    router = null;
    vuetify = null;

    constructor(router, vuetify) {
        this.router = router;
        this.vuetify = vuetify.framework;
        let deviceId = storage.local('device-id');
        if (!deviceId) {
            deviceId = v4();
            storage.local('device-id', deviceId);
        }
        this.axios = axios.create({
            baseURL: '/api/',
            timeout: 30000,
            headers: {
                'x-device-id': deviceId,
            },
        })
    }

    appId(id) {
        const parts = id.split(':');
        return {
            ns: parts[0] !== '_' ? parts[0] : '',
            kind: parts[1],
            name: parts[3] ? ':'+parts[3] : parts[2],
        }
    }

    request(req, cb) {
        req.headers = {...req.headers, 'x-device-size': this.vuetify.breakpoint.name};
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

    del(url, cb) {
        this.request({method: 'delete', url}, cb);
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

    delProject(projectId, cb) {
        this.del(`project/${projectId}`, cb);
    }

    projectPath(subPath) {
        const projectId = this.router.currentRoute.params.projectId;
        return `project/${projectId}/${subPath}`;
    }

    getStatus(cb) {
        this.get(this.projectPath(`status`), cb);
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
