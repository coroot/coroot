import Vue from "vue";
import VueRouter from "vue-router";
import vuetify from '@/plugins/vuetify';
import '@/plugins/resize';
import moment from 'moment';
import Api from "@/api";
import App from "@/App";
import Overview from "@/views/Overview";
import Application from "@/views/Application";
import Node from "@/views/Node";

Vue.config.productionTip = false;

Vue.use(VueRouter);
const router = new VueRouter({
  mode: "history",
  routes: [
      {path: "/", name: "overview", component: Overview},
      {path: "/app/:id/:dashboard?", name: "application", component: Application, props: true},
      {path: "/node/:name", name: "node", component: Node, props: true},
  ],
});


Vue.prototype.$moment = moment;
Vue.prototype.$api = new Api(router);

new Vue({
  router,
  vuetify,
  render: (h) => h(App)
}).$mount("#app");
