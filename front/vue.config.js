const { defineConfig } = require("@vue/cli-service");
module.exports = defineConfig({
  publicPath: '{{.BasePath}}static/',
  transpileDependencies: [
    'vuetify',
    '@prometheus-io/codemirror-promql',
    'sql-formatter'
  ],
});
