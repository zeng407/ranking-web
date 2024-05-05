import Vue from 'vue';
import VueI18n from 'vue-i18n';
Vue.use(VueI18n);

import en from '../lang/en.json';
import customZh_TW from '../lang/zh-TW.json';
import veeValidateZh_TW from "vee-validate/dist/locale/zh_TW.json";

const zh_TW = {
  ...customZh_TW,
  ...veeValidateZh_TW
};

const i18n = new VueI18n({
    locale: document.documentElement.lang || 'en',
    messages: {
        'en': en,
        'zh-TW': zh_TW,
    }
});
export {i18n}