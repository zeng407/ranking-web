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
 * use svg+js method instead of webfonts
 */
// import '@fortawesome/fontawesome-free/js/all.js';

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

import {PaginationPlugin, AlertPlugin, TabsPlugin, PopoverPlugin, VBTooltipPlugin, TooltipPlugin} from 'bootstrap-vue';
// Make BootstrapVue available throughout your project
Vue.use(PaginationPlugin);
Vue.use(AlertPlugin);
Vue.use(TabsPlugin);
Vue.use(PopoverPlugin);
Vue.use(VBTooltipPlugin);
Vue.use(TooltipPlugin);

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

require('./plugins/vee-validate');

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
    return moment(value).format('Y-M-D');
});
Vue.filter('datetime', function (value) {
    return moment(value).format('Y-M-D HH:mm:ss');
});
Vue.filter('moment', function (value, format) {
    return moment(value).format(format);
});
Vue.filter('formNow', function (value) {
    const locale = document.documentElement.lang || 'en';
    moment.locale(locale);
    return moment(value).fromNow();
});
Vue.filter('cut', function(value, maxLength) {
    if (!value) return '';
    if (value.length <= maxLength) return value;
    return value.slice(0, maxLength) + '...';
  });

/* Viewer */
import 'viewerjs/dist/viewer.css'
import VueViewer from 'v-viewer'
Vue.use(VueViewer)

// EventBus
Vue.prototype.$bus = new Vue(); // Define EventBus globally


// handle axios response

// axios.interceptors.response.use(response => {
//     // Check for the 'cf-mitigated' header in the response
//     const cfMitigated = response.headers.get('cf-mitigated');
//     if (cfMitigated === 'challenge') {
//       console.log('Response was mitigated by Cloudflare:', cfMitigated);
//       // Handle the mitigation here. For example, you might want to log it,
//       // show a notification to the user, or perform some other action.

//     }
//     // Always return the response so the calling code receives it
//     return response;
//   }, error => {
//     // Do something with response error
//     return Promise.reject(error);
//   });

const app = new Vue({
    el: '#app',
    i18n
});
