import Vue from "vue";
import VueRouter from "vue-router";
import vuetify from '@/plugins/vuetify';
import '@/plugins/resize';
import moment from 'moment';
import Api from "@/api";
import App from "@/App";
import Overview from "@/views/Overview";
import Application from "@/views/Application";

Vue.config.productionTip = false;

Vue.use(VueRouter);
const router = new VueRouter({
  mode: "history",
  routes: [
      {path: "/", name: "overview", component: Overview},
      {path: "/app/:id", name: "application", component: Application, props: true},
  ],
});


Vue.prototype.$moment = moment;
Vue.prototype.$api = new Api(router);

new Vue({
  router,
  vuetify,
  render: (h) => h(App)
}).$mount("#app");
