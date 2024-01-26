import axios from 'axios';
import * as storage from '@/utils/storage';
import { v4 } from 'uuid';

const defaultErrorMessage = 'Something went wrong, please try again later.';
const timeoutErrorMessage = 'Request timed out.';

export default class Api {
    axios = null;
    router = null;
    vuetify = null;
    deviceId = '';
    basePath = '';

    context = {
        status: {},
        search: {},
    };

    constructor(router, vuetify, basePath) {
        this.router = router;
        this.vuetify = vuetify.framework;
        this.deviceId = storage.local('device-id');
        if (!this.deviceId) {
            this.deviceId = v4();
            storage.local('device-id', this.deviceId);
        }
        this.basePath = basePath;
        this.axios = axios.create({
            baseURL: this.basePath + 'api/',
            timeout: 60000,
            timeoutErrorMessage: timeoutErrorMessage,
        });
    }

    stats(type, data) {
        const event = {
            ...data,
            type,
            device_id: this.deviceId,
            device_size: this.vuetify.breakpoint.name,
        };
        navigator.sendBeacon(this.basePath + 'stats', JSON.stringify(event));
    }

    request(req, cb) {
        this.axios(req)
            .then((response) => {
                if (response.data.context) {
                    this.context.status = response.data.context.status;
                    this.context.search = response.data.context.search;
                }
                try {
                    cb(response.data.data || response.data, '');
                } catch (e) {
                    console.error(e);
                }
            })
            .catch((error) => {
                const err = (error.response && error.response.data && error.response.data.trim()) || error.message || defaultErrorMessage;
                cb(null, err);
            });
    }

    get(url, args, cb) {
        const { from, to } = this.router.currentRoute.query;
        const params = { ...args, from, to };
        this.request({ method: 'get', url, params }, cb);
    }

    put(url, data, cb) {
        this.request({ method: 'put', url, data }, cb);
    }

    post(url, data, cb) {
        this.request({ method: 'post', url, data }, cb);
    }

    del(url, cb) {
        this.request({ method: 'delete', url }, cb);
    }

    getProjects(cb) {
        this.get(`projects`, {}, cb);
    }

    getProject(projectId, cb) {
        this.get(`project/${projectId || ''}`, {}, cb);
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
        this.get(this.projectPath(`status`), {}, cb);
    }

    setStatus(form, cb) {
        this.post(this.projectPath(`status`), form, cb);
    }

    getOverview(view, cb) {
        this.get(this.projectPath(`overview/${view}`), {}, cb);
    }

    getCheckConfigs(cb) {
        this.get(this.projectPath(`configs`), {}, cb);
    }

    getApplicationCategories(cb) {
        this.get(this.projectPath(`categories`), {}, cb);
    }

    saveApplicationCategory(form, cb) {
        this.post(this.projectPath(`categories`), form, cb);
    }

    getIntegrations(type, cb) {
        this.get(this.projectPath(`integrations${type ? '/' + type : ''}`), {}, cb);
    }

    saveIntegrations(type, action, form, cb) {
        const path = this.projectPath(`integrations${type ? '/' + type : ''}`);
        switch (action) {
            case 'save':
                this.put(path, form, cb);
                return;
            case 'del':
                this.del(path, cb);
                return;
            case 'test':
                this.post(path, form, cb);
                return;
        }
    }

    getApplication(appId, cb) {
        this.get(this.projectPath(`app/${appId}`), {}, cb);
    }

    getCheckConfig(appId, checkId, cb) {
        this.get(this.projectPath(`app/${appId}/check/${checkId}/config`), {}, cb);
    }

    saveCheckConfig(appId, checkId, form, cb) {
        this.post(this.projectPath(`app/${appId}/check/${checkId}/config`), form, cb);
    }

    getProfile(appId, query, cb) {
        this.get(this.projectPath(`app/${appId}/profile`), { query }, cb);
    }

    saveProfileSettings(appId, form, cb) {
        this.post(this.projectPath(`app/${appId}/profile`), form, cb);
    }

    getTracing(appId, trace, cb) {
        this.get(this.projectPath(`app/${appId}/tracing`), { trace }, cb);
    }

    saveTracingSettings(appId, form, cb) {
        this.post(this.projectPath(`app/${appId}/tracing`), form, cb);
    }

    getLogs(appId, query, cb) {
        this.get(this.projectPath(`app/${appId}/logs`), { query }, cb);
    }

    saveLogsSettings(appId, form, cb) {
        this.post(this.projectPath(`app/${appId}/logs`), form, cb);
    }

    getNode(nodeName, cb) {
        this.get(this.projectPath(`node/${nodeName}`), {}, cb);
    }

    getPrometheusCompleteConfiguration() {
        return {
            remote: {
                apiPrefix: this.basePath + 'api/' + this.projectPath('prom') + '/api/v1',
            },
        };
    }
}
