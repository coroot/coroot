import Vue from "vue";
import VueRouter from "vue-router";
import vuetify from '@/plugins/vuetify';
import '@/plugins/resize';
import pluralize from 'pluralize';
import events from '@/utils/events';
import * as validators from "@/utils/validators";
import * as storage from "@/utils/storage";
import * as format from '@/utils/format';
import Api from "@/api";
import App from "@/App";
import Project from "@/views/Project";
import Overview from "@/views/Overview";
import Application from "@/views/Application";
import Node from "@/views/Node";
import Welcome from "@/views/Welcome";

Vue.config.productionTip = false;

Vue.use(VueRouter);
const router = new VueRouter({
    mode: "history",
    routes: [
        {path: "/p/new", name: "project_new", component: Project},
        {path: "/p/:projectId/settings", name: "project_settings", component: Project, props: true},
        {path: "/p/:projectId", name: "overview", component: Overview, props: true},
        {path: "/p/:projectId/app/:id/:report?", name: "application", component: Application, props: true},
        {path: "/p/:projectId/node/:name", name: "node", component: Node, props: true},
        {path: '/welcome', name: 'welcome', component: Welcome},
        {path: '/', name: 'index', component: App},
        {path: '*', redirect: {name: 'index'}},
    ],
    scrollBehavior(to) {
        if (to.hash) {
            return new Promise((resolve) => {
                setTimeout(() => {
                    resolve({ selector: to.hash, behavior: 'smooth' });
                }, 300);
            });
        }
    }
});

const api = new Api(router, vuetify);

router.afterEach((to, from) => {
    if (to.params.projectId !== from.params.projectId || JSON.stringify(to.query) !== JSON.stringify(from.query)) {
        events.emit('refresh');
    }
    const m = to.matched[0];
    if (m) {
        let p = m.path;
        p = p.replace(':report?', to.params.report || '')
        p = p.replaceAll(':', '$');
        api.stats("route-open", {path: p});
    }
});

Vue.prototype.$events = events;
Vue.prototype.$format = format;
Vue.prototype.$pluralize = pluralize;
Vue.prototype.$api = api;
Vue.prototype.$validators = validators;
Vue.prototype.$storage = storage;
Vue.prototype.$coroot = window.coroot;

new Vue({
  router,
  vuetify,
  render: (h) => h(App)
}).$mount("#app");
