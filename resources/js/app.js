/**
 * First we will load all of this project's JavaScript dependencies which
 * includes Vue and other libraries. It is a great starting point when
 * building robust, powerful web applications using Vue and Laravel.
 */

require('./bootstrap');

window.Vue = require('vue').default;

/**
 * The following block of code may be used to automatically register your
 * Vue components. It will recursively scan this directory for the Vue
 * components and automatically register them with their "basename".
 *
 * Eg. ./components/ExampleComponent.vue -> <example-component></example-component>
 */

const files = require.context('./', true, /\.vue$/i);
files.keys().map(key => Vue.component(key.split('/').pop().split('.')[0], files(key).default));


// Vue.component('example-component', require('./components/ExampleComponent.vue').default);

/**
 * Next, we will create a fresh Vue application instance and attach it to
 * the page. Then, you may begin adding components to this application
 * or customize the JavaScript scaffolding to fit your unique needs.
 */

/**
 * validation lang
 */
import {ValidationProvider, ValidationObserver, extend, localize} from 'vee-validate';
import en from 'vee-validate/dist/locale/en';
import zh_TW from 'vee-validate/dist/locale/zh_TW';

localize({en, zh_TW});

/**
 * register validation rule
 */
import {required, min_value} from 'vee-validate/dist/rules';

Vue.component('ValidationProvider', ValidationProvider);
Vue.component('ValidationObserver', ValidationObserver);
extend('required', required);
extend('min_value', min_value);

/**
 * import datetime plugin
 */
import moment from 'moment';

Vue.prototype.moment = moment;
window.moment = moment;

/**
 * Cookie Tool
 */
import VueCookies from 'vue-cookies';

Vue.use(VueCookies);

/**
 * BootstrapVue
 */

import {PaginationPlugin, AlertPlugin, TabsPlugin, VBPopover, VBTooltipPlugin} from 'bootstrap-vue';
// Make BootstrapVue available throughout your project
Vue.use(PaginationPlugin);
Vue.use(AlertPlugin);
Vue.use(TabsPlugin);
Vue.use(VBPopover);
Vue.use(VBTooltipPlugin)
// Optionally install the BootstrapVue icon components plugin
// Vue.use(IconsPlugin);

/**
 * YoutubeIframe
 */
import VueYoutube from 'vue-youtube';

Vue.use(VueYoutube);

/**
 * i18n
 */
import {i18n} from './plugins/vue-i18n';


/**
 * Filter
 */
Vue.filter('percent', function (value) {
    if (value) {
        return value + '%';
    }
    return null;
});
Vue.filter('date', function (value) {
    return moment(value).format('Y/M/D');
});
Vue.filter('datetime', function (value) {
    return moment(value).format('Y/M/D HH:mm:ss');
});
Vue.filter('moment', function (value, format) {
    return moment(value).format(format);
});
Vue.filter('formNow', function (value, locale = 'zh-tw') {
    moment.locale(locale || 'en');
    return moment(value).fromNow();
});


Vue.directive('b-popover', VBPopover);

const app = new Vue({
    el: '#app',
    i18n
});
