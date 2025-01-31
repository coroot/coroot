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
            theme: storage.local('theme') || '',
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
                const err = error.response && error.response.data && error.response.data.trim();
                if (error.response && error.response.status === 302) {
                    window.location = err;
                }
                if (error.response && error.response.status === 401) {
                    const r = this.router.currentRoute;
                    const action = err || undefined;
                    if (!r.meta.anonymous || r.query.action !== action) {
                        const next = r.fullPath !== '/' && r.name !== 'login' ? r.fullPath : undefined;
                        this.router.push({ name: 'login', query: { action, next } }).catch((err) => err);
                    }
                }
                cb(null, err || error.message || defaultErrorMessage);
            });
    }

    get(url, args, cb) {
        const { from, to, incident, rcaFrom, rcaTo } = this.router.currentRoute.query;
        const params = { ...args, from, to, incident, rcaFrom, rcaTo };
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

    user(form, cb) {
        if (form) {
            this.post(`user`, form, cb);
        } else {
            this.get(`user`, {}, cb);
        }
    }

    login(form, cb) {
        this.post(`login`, form, cb);
    }

    logout(cb) {
        this.post(`logout`, null, cb);
    }

    users(form, cb) {
        if (form) {
            this.post(`users`, form, cb);
        } else {
            this.get(`users`, {}, cb);
        }
    }

    roles(form, cb) {
        if (form) {
            this.post(`roles`, form, cb);
        } else {
            this.get(`roles`, {}, cb);
        }
    }

    sso(form, cb) {
        if (form) {
            this.post(`sso`, form, cb);
        } else {
            this.get(`sso`, {}, cb);
        }
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

    apiKeys(form, cb) {
        const url = this.projectPath('api_keys');
        if (form) {
            this.post(url, form, cb);
        } else {
            this.get(url, {}, cb);
        }
    }

    getOverview(view, query, cb) {
        this.get(this.projectPath(`overview/${view}`), { query }, cb);
    }

    getInspections(cb) {
        this.get(this.projectPath(`inspections`), {}, cb);
    }

    getApplicationCategories(cb) {
        this.get(this.projectPath(`categories`), {}, cb);
    }

    saveApplicationCategory(form, cb) {
        this.post(this.projectPath(`categories`), form, cb);
    }

    getCustomApplications(cb) {
        this.get(this.projectPath(`custom_applications`), {}, cb);
    }

    saveCustomApplication(form, cb) {
        this.post(this.projectPath(`custom_applications`), form, cb);
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

    getIncident(key, cb) {
        this.get(this.projectPath(`incident/${key}`), {}, cb);
    }

    getRCA(appId, cb) {
        this.get(this.projectPath(`app/${appId}/rca`), {}, cb);
    }

    getInspectionConfig(appId, type, cb) {
        this.get(this.projectPath(`app/${appId}/inspection/${type}/config`), {}, cb);
    }

    saveInspectionConfig(appId, type, form, cb) {
        this.post(this.projectPath(`app/${appId}/inspection/${type}/config`), form, cb);
    }

    getInstrumentation(appId, type, cb) {
        this.get(this.projectPath(`app/${appId}/instrumentation/${type}`), {}, cb);
    }

    saveInstrumentationSettings(appId, type, form, cb) {
        this.post(this.projectPath(`app/${appId}/instrumentation/${type}`), form, cb);
    }

    getProfiling(appId, query, cb) {
        this.get(this.projectPath(`app/${appId}/profiling`), { query }, cb);
    }

    saveProfilingSettings(appId, form, cb) {
        this.post(this.projectPath(`app/${appId}/profiling`), form, cb);
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

    risks(appId, form, cb) {
        this.post(this.projectPath(`app/${appId}/risks`), form, cb);
    }

    getPrometheusCompleteConfiguration() {
        return {
            remote: {
                apiPrefix: this.basePath + 'api/' + this.projectPath('prom') + '/api/v1',
            },
        };
    }
}
