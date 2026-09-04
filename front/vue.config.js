const { defineConfig } = require("@vue/cli-service");

// vue-loader v15 emits `import style0 from '...?vue&type=style...'` for every <style> block, and
// this import is never read in production builds. Since webpack 5.110.2, such imports of a missing
// export are reported as warnings; drop them so the build output stays readable.
const ignoreVueStyleImportWarnings = {
  apply(compiler) {
    compiler.hooks.thisCompilation.tap('IgnoreVueStyleImportWarnings', (compilation) => {
      compilation.hooks.afterSeal.tap('IgnoreVueStyleImportWarnings', () => {
        compilation.warnings = compilation.warnings.filter(
          (w) => !/export 'default' \(imported as 'style\d+'\) was not found/.test(w.message),
        );
      });
    });
  },
};

module.exports = defineConfig({
  publicPath: '{{.BasePath}}static/',
  transpileDependencies: [
    'vuetify',
    '@prometheus-io/codemirror-promql',
    'sql-formatter'
  ],
  configureWebpack: {
    plugins: [ignoreVueStyleImportWarnings],
  },
});
