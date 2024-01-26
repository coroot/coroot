import Vue from 'vue';
import VueRouter from 'vue-router';
import vuetify from '@/plugins/vuetify';
import '@/plugins/resize';
import '@/plugins/highlight';
import pluralize from 'pluralize';
import events from '@/utils/events';
import Utils from '@/utils/utils';
import * as validators from '@/utils/validators';
import * as storage from '@/utils/storage';
import * as format from '@/utils/format';
import Api from '@/api';
import App from '@/App';
import Project from '@/views/Project';
import Overview from '@/views/Overview';
import Application from '@/views/Application';
import Node from '@/views/Node';
import Welcome from '@/views/Welcome';

Vue.config.productionTip = false;
Vue.config.devtools = false;

const config = window.coroot;

Vue.use(VueRouter);
const router = new VueRouter({
    mode: 'history',
    base: config.base_path,
    routes: [
        { path: '/p/new/:tab?', name: 'project_new', component: Project },
        { path: '/p/:projectId/settings/:tab?', name: 'project_settings', component: Project, props: true, meta: { stats: { param: 'tab' } } },
        { path: '/p/:projectId/:view?', name: 'overview', component: Overview, props: true, meta: { stats: { param: 'view' } } },
        { path: '/p/:projectId/app/:id/:report?', name: 'application', component: Application, props: true, meta: { stats: { param: 'report' } } },
        { path: '/p/:projectId/node/:name', name: 'node', component: Node, props: true },
        { path: '/welcome', name: 'welcome', component: Welcome },
        { path: '/', name: 'index', component: App },
        { path: '*', redirect: { name: 'index' } },
    ],
    scrollBehavior(to) {
        if (to.hash) {
            try {
                document.querySelector(to.hash);
                return new Promise((resolve) => {
                    setTimeout(() => {
                        resolve({ selector: to.hash, behavior: 'smooth' });
                    }, 300);
                });
            } catch {
                //
            }
        }
    },
});

const api = new Api(router, vuetify, config.base_path);

router.afterEach((to) => {
    if (to.matched[0]) {
        let p = to.matched[0].path;
        if (to.meta.stats && to.meta.stats.param) {
            p = p.replace(':' + to.meta.stats.param, to.params[to.meta.stats.param] || '');
        }
        p = p.replaceAll(':', '$');
        if (to.name === 'application' && to.params.report === 'Profiling' && to.query.query) {
            try {
                const q = JSON.parse(to.query.query);
                p += `${q.type || ''}:${q.mode || ''}:${Number(q.from) || Number(q.to) ? 'ts' : ''}`;
            } catch {
                //
            }
        }
        if (to.name === 'application' && to.params.report === 'Tracing' && to.query.trace) {
            const [type, id, ts, dur] = to.query.trace.split(':');
            p += `${type}:${id ? 'id' : ''}:${ts !== '-' ? 'ts' : ''}:${dur}`;
        }
        if (to.name === 'application' && to.params.report === 'Logs' && to.query.query) {
            try {
                const q = JSON.parse(to.query.query);
                p += `${q.source || ''}:${q.view || ''}:${q.severity || ''}:${q.hash ? 'hash' : ''}:${q.search ? 'search' : ''}`;
            } catch {
                //
            }
        }
        api.stats('route-open', { path: p });
    }
});

Vue.prototype.$events = events;
Vue.prototype.$format = format;
Vue.prototype.$pluralize = pluralize;
Vue.prototype.$api = api;
Vue.prototype.$utils = new Utils(router);
Vue.prototype.$validators = validators;
Vue.prototype.$storage = storage;
Vue.prototype.$coroot = config;

new Vue({
    router,
    vuetify,
    render: (h) => h(App),
}).$mount('#app');
