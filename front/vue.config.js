const { defineConfig } = require("@vue/cli-service");
module.exports = defineConfig({
  publicPath: '/static/',
  transpileDependencies: [
    'vuetify'
  ],
});
