import Vue from "vue";
import VueRouter from "vue-router";
import Vuetify from "vuetify/lib";
import "@mdi/font/css/materialdesignicons.css";
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

Vue.use(Vuetify);
const vuetify = new Vuetify({
  icons: {
    iconfont: 'mdi',
  },
});

Vue.prototype.$api = new Api();

new Vue({
  router,
  vuetify,
  render: (h) => h(App)
}).$mount("#app");
