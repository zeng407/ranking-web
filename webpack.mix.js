const mix = require('laravel-mix');

/*
 |--------------------------------------------------------------------------
 | Mix Asset Management
 |--------------------------------------------------------------------------
 |
 | Mix provides a clean, fluent API for defining some Webpack build steps
 | for your Laravel application. By default, we are compiling the Sass
 | file for the application as well as bundling up all the JS files.
 |
 */

mix.js('resources/js/app.js', 'public/js')
    .vue()
    .sass('resources/sass/app.scss', 'public/css');

// Only include source maps when not in production
if (!mix.inProduction()) {
    mix.sourceMaps();
}

if(mix.inProduction()) {
    mix.sass('resources/sass/prod.scss', 'public/css');
}else{
    mix.sass('resources/sass/local.scss', 'public/css');
}

mix.version();
