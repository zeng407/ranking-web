import Vue from 'vue';
import VueI18n from 'vue-i18n';
Vue.use(VueI18n);

import en from '../lang/en.json';
import zh_TW from '../lang/zh-TW.json';

const i18n = new VueI18n({
    locale: document.documentElement.lang || 'en',
    //this.$i18n.locale // 通過切換locale的值來實現語言切換
    messages: {
        'en': en,
        'zh-TW': zh_TW
    }
});
export {i18n}
