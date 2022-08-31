import Vue from "vue";
import VueRouter from "vue-router";
import vuetify from '@/plugins/vuetify';
import '@/plugins/resize';
import moment from 'moment';
import * as validators from "@/utils/validators";
import Api from "@/api";
import App from "@/App";
import Project from "@/views/Project";
import Overview from "@/views/Overview";
import Application from "@/views/Application";
import Node from "@/views/Node";

Vue.config.productionTip = false;

Vue.use(VueRouter);
const router = new VueRouter({
  mode: "history",
  routes: [
      {path: "/p/new", name: "project_new", component: Project},
      {path: "/p/:projectId/settings", name: "project_settings", component: Project, props: true},
      {path: "/p/:projectId", name: "overview", component: Overview, props: true},
      {path: "/p/:projectId/app/:id/:dashboard?", name: "application", component: Application, props: true},
      {path: "/p/:projectId/node/:name", name: "node", component: Node, props: true},
      {path: '/', name: 'index', component: App},
      {path: '*', redirect: {name: 'index'}},
  ],
});


Vue.prototype.$moment = moment;
Vue.prototype.$api = new Api(router);
Vue.prototype.$validators = validators;

new Vue({
  router,
  vuetify,
  render: (h) => h(App)
}).$mount("#app");
